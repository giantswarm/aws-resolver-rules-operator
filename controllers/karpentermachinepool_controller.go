package controllers

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"time"

	"github.com/blang/semver/v4"
	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	capa "sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	capalogger "sigs.k8s.io/cluster-api-provider-aws/v2/pkg/logger"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/remote"
	capiexp "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	capiutilexp "sigs.k8s.io/cluster-api/exp/util"
	capiutil "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/aws-resolver-rules-operator/api/v1alpha1"
	"github.com/aws-resolver-rules-operator/pkg/conditions"
	"github.com/aws-resolver-rules-operator/pkg/resolver"
)

const (
	// BootstrapDataHashAnnotation stores the SHA256 hash of the bootstrap data
	// to detect when userdata changes and needs to be re-uploaded to S3
	BootstrapDataHashAnnotation = "giantswarm.io/userdata-hash"

	// EC2NodeClassAPIGroup is the API group for Karpenter EC2NodeClass resources
	EC2NodeClassAPIGroup = "karpenter.k8s.aws"

	// KarpenterFinalizer ensures proper cleanup of Karpenter resources and EC2 instances
	// before allowing the KarpenterMachinePool to be deleted
	KarpenterFinalizer = "capa-operator.finalizers.giantswarm.io/karpenter-controller"

	// S3ObjectPrefix is the S3 path prefix where bootstrap data is stored
	// Format: s3://<bucket>/<prefix>/<machine-pool-name>
	S3ObjectPrefix = "karpenter-machine-pool"
)

type KarpenterMachinePoolReconciler struct {
	awsClients resolver.AWSClients
	client     client.Client
	// clusterClientGetter is used to create a kubernetes client targeting the workload cluster
	clusterClientGetter remote.ClusterClientGetter
}

func NewKarpenterMachinepoolReconciler(client client.Client, clusterClientGetter remote.ClusterClientGetter, awsClients resolver.AWSClients) *KarpenterMachinePoolReconciler {
	return &KarpenterMachinePoolReconciler{awsClients: awsClients, client: client, clusterClientGetter: clusterClientGetter}
}

// Reconcile reconciles KarpenterMachinePool CRs, which represent cluster node pools that will be managed by karpenter.
// KarpenterMachinePoolReconciler reconciles KarpenterMachinePool objects by:
// 1. Creating Karpenter NodePool and EC2NodeClass resources in workload clusters
// 2. Managing bootstrap data upload to S3 for node initialization
// 3. Enforcing Kubernetes version skew policies
// 4. Cleaning up EC2 instances when clusters are deleted
func (r *KarpenterMachinePoolReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling")
	defer logger.Info("Done reconciling")

	karpenterMachinePool := &v1alpha1.KarpenterMachinePool{}
	if err := r.client.Get(ctx, req.NamespacedName, karpenterMachinePool); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	machinePool, err := capiutilexp.GetOwnerMachinePool(ctx, r.client, karpenterMachinePool.ObjectMeta)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get MachinePool owning the KarpenterMachinePool: %w", err)
	}
	if machinePool == nil {
		// We return early, we need to wait until the MachinePool Controller sets the OwnerRef on the KarpenterMachinePool.
		// We don't need to requeue though, because setting the OwnerRef on the KarpenterMachinePool will trigger a new reconciliation.
		logger.Info("MachinePool Controller has not yet set OwnerRef on the KarpenterMachinePool, returning early")
		return reconcile.Result{}, nil
	}
	logger = logger.WithValues("machinePool", machinePool.Name)

	// Bootstrap data must be available before we can proceed with creating Karpenter resources
	if machinePool.Spec.Template.Spec.Bootstrap.DataSecretName == nil {
		logger.Info("Bootstrap data secret reference is not yet available")
		return reconcile.Result{RequeueAfter: time.Duration(10) * time.Second}, nil
	}

	logger = logger.WithValues("dataSecretName", *machinePool.Spec.Template.Spec.Bootstrap.DataSecretName)

	cluster, err := capiutil.GetClusterFromMetadata(ctx, r.client, machinePool.ObjectMeta)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get Cluster owning the MachinePool that owns the KarpenterMachinePool: %w", err)
	}

	logger = logger.WithValues("cluster", cluster.GetName())

	if annotations.IsPaused(cluster, karpenterMachinePool) {
		logger.Info("Reconciliation is paused for this object")
		return ctrl.Result{}, nil
	}

	awsCluster := &capa.AWSCluster{}
	if err := r.client.Get(ctx, client.ObjectKey{Namespace: cluster.Spec.InfrastructureRef.Namespace, Name: cluster.Spec.InfrastructureRef.Name}, awsCluster); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get AWSCluster referenced in Cluster.spec.infrastructureRef: %w", err)
	}

	if annotations.IsPaused(cluster, awsCluster) {
		logger.Info("Reconciliation is paused for this object")
		return ctrl.Result{}, nil
	}

	// S3 bucket is required for storing bootstrap data that Karpenter nodes will fetch
	if awsCluster.Spec.S3Bucket == nil {
		return reconcile.Result{}, errors.New("a cluster wide object storage configured at `AWSCluster.spec.s3Bucket` is required")
	}

	// Get AWS credentials for S3 and EC2 operations
	roleIdentity := &capa.AWSClusterRoleIdentity{}
	if err = r.client.Get(ctx, client.ObjectKey{Name: awsCluster.Spec.IdentityRef.Name}, roleIdentity); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get AWSClusterRoleIdentity referenced in AWSCluster: %w", err)
	}

	// Handle deletion: cleanup EC2 instances and Karpenter resources
	if !karpenterMachinePool.GetDeletionTimestamp().IsZero() {
		return r.reconcileDelete(ctx, logger, cluster, awsCluster, karpenterMachinePool, roleIdentity)
	}

	// Create a deep copy of the reconciled object so we can change it
	karpenterMachinePoolFinalizerCopy := karpenterMachinePool.DeepCopy()

	// Initialize conditions - mark as initializing until all steps complete
	conditions.MarkKarpenterMachinePoolNotReady(karpenterMachinePool, conditions.InitializingReason, "KarpenterMachinePool is being initialized")

	// Add finalizer to ensure proper cleanup sequence
	updated := controllerutil.AddFinalizer(karpenterMachinePool, KarpenterFinalizer)
	if updated {
		if err := r.client.Patch(ctx, karpenterMachinePool, client.MergeFrom(karpenterMachinePoolFinalizerCopy)); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer to KarpenterMachinePool: %w", err)
		}
	}

	// Create or update Karpenter custom resources in the workload cluster.
	if err := r.createOrUpdateKarpenterResources(ctx, logger, cluster, awsCluster, karpenterMachinePool, machinePool); err != nil {
		logger.Error(err, "failed to create or update Karpenter custom resources in the workload cluster")

		// Ensure conditions are persisted even when errors occur
		if statusErr := r.client.Status().Update(ctx, karpenterMachinePool); statusErr != nil {
			logger.Error(statusErr, "failed to update karpenterMachinePool status with error conditions")
		}

		return reconcile.Result{}, err
	}

	// Reconcile bootstrap data - fetch secret and upload to S3 if changed
	if err := r.reconcileMachinePoolBootstrapUserData(ctx, logger, awsCluster, karpenterMachinePool, *machinePool.Spec.Template.Spec.Bootstrap.DataSecretName, roleIdentity); err != nil {
		conditions.MarkBootstrapDataNotReady(karpenterMachinePool, conditions.BootstrapDataUploadFailedReason, fmt.Sprintf("Failed to reconcile bootstrap data: %v", err))

		// Ensure conditions are persisted even when errors occur
		if statusErr := r.client.Status().Update(ctx, karpenterMachinePool); statusErr != nil {
			logger.Error(statusErr, "failed to update karpenterMachinePool status with bootstrap error conditions")
		}

		return reconcile.Result{}, err
	}
	conditions.MarkBootstrapDataReady(karpenterMachinePool)

	// Persist bootstrap data success condition immediately
	if statusErr := r.client.Status().Update(ctx, karpenterMachinePool); statusErr != nil {
		logger.Error(statusErr, "failed to update karpenterMachinePool status with bootstrap data success condition")
		return reconcile.Result{}, fmt.Errorf("failed to persist bootstrap data success condition: %w", statusErr)
	}

	// Update status with current node information from the workload cluster
	if err := r.saveKarpenterInstancesToStatus(ctx, logger, cluster, karpenterMachinePool, machinePool); err != nil {
		logger.Error(err, "failed to save Karpenter instances to status")

		// Ensure conditions are persisted even when errors occur
		if statusErr := r.client.Status().Update(ctx, karpenterMachinePool); statusErr != nil {
			logger.Error(statusErr, "failed to update karpenterMachinePool status with conditions before returning error")
		}

		return reconcile.Result{}, err
	}

	// Mark the KarpenterMachinePool as ready when all conditions are satisfied
	conditions.MarkKarpenterMachinePoolReady(karpenterMachinePool)

	// Update the status to persist the Ready condition
	if err := r.client.Status().Update(ctx, karpenterMachinePool); err != nil {
		logger.Error(err, "failed to update karpenterMachinePool status with Ready condition")
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

// saveKarpenterInstancesToStatus updates the KarpenterMachinePool and parent MachinePool with current node information
// from the workload cluster, including replica counts and provider ID lists.
func (r *KarpenterMachinePoolReconciler) saveKarpenterInstancesToStatus(ctx context.Context, logger logr.Logger, cluster *capi.Cluster, karpenterMachinePool *v1alpha1.KarpenterMachinePool, machinePool *capiexp.MachinePool) error {
	// Create a copy for tracking changes
	karpenterMachinePoolCopy := karpenterMachinePool.DeepCopy()

	// Get current node information from the workload cluster
	providerIDList, numberOfNodeClaims, err := r.computeProviderIDListFromNodeClaimsInWorkloadCluster(ctx, logger, cluster)
	if err != nil {
		return err
	}

	// Update KarpenterMachinePool status with current replica count
	karpenterMachinePool.Status.Replicas = numberOfNodeClaims

	logger.Info("Found NodeClaims in workload cluster, patching KarpenterMachinePool", "numberOfNodeClaims", numberOfNodeClaims, "providerIDList", providerIDList)

	// Patch the status with the updated replica count
	if err := r.client.Status().Patch(ctx, karpenterMachinePool, client.MergeFrom(karpenterMachinePoolCopy), client.FieldOwner("karpentermachinepool-controller")); err != nil {
		logger.Error(err, "failed to patch karpenterMachinePool.status.Replicas")
		return err
	}

	// Update KarpenterMachinePool spec with current provider ID list
	karpenterMachinePool.Spec.ProviderIDList = providerIDList

	// Patch the spec with the updated provider ID list
	if err := r.client.Patch(ctx, karpenterMachinePool, client.MergeFrom(karpenterMachinePoolCopy), client.FieldOwner("karpentermachinepool-controller")); err != nil {
		logger.Error(err, "failed to patch karpenterMachinePool.spec.providerIDList")
		return err
	}

	// Update the parent MachinePool replica count to match actual node claims
	if machinePool.Spec.Replicas == nil || *machinePool.Spec.Replicas != numberOfNodeClaims {
		machinePoolCopy := machinePool.DeepCopy()
		machinePool.Spec.Replicas = &numberOfNodeClaims
		if err := r.client.Patch(ctx, machinePool, client.MergeFrom(machinePoolCopy), client.FieldOwner("karpenter-machinepool-controller")); err != nil {
			logger.Error(err, "failed to patch MachinePool.spec.replicas")
			return err
		}
	}

	return nil
}

// reconcileMachinePoolBootstrapUserData handles the bootstrap user data reconciliation process.
// It fetches the bootstrap secret, checks if the data has changed, and uploads it to S3 if needed.
// It also updates the hash annotation to track the current bootstrap data version.
func (r *KarpenterMachinePoolReconciler) reconcileMachinePoolBootstrapUserData(ctx context.Context, logger logr.Logger, awsCluster *capa.AWSCluster, karpenterMachinePool *v1alpha1.KarpenterMachinePool, dataSecretName string, roleIdentity *capa.AWSClusterRoleIdentity) error {
	// Get the bootstrap secret containing userdata for node initialization
	bootstrapSecret := &v1.Secret{}
	if err := r.client.Get(ctx, client.ObjectKey{Namespace: karpenterMachinePool.Namespace, Name: dataSecretName}, bootstrapSecret); err != nil {
		if k8serrors.IsNotFound(err) {
			conditions.MarkBootstrapDataNotReady(karpenterMachinePool, conditions.BootstrapDataSecretNotFoundReason, fmt.Sprintf("Bootstrap secret %s not found", dataSecretName))
		} else {
			conditions.MarkBootstrapDataNotReady(karpenterMachinePool, conditions.BootstrapDataUploadFailedReason, fmt.Sprintf("Failed to get bootstrap secret %s: %v", dataSecretName, err))
		}
		return fmt.Errorf("failed to get bootstrap secret in MachinePool.spec.template.spec.bootstrap.dataSecretName: %w", err)
	}

	bootstrapSecretValue, ok := bootstrapSecret.Data["value"]
	if !ok {
		conditions.MarkBootstrapDataNotReady(karpenterMachinePool, conditions.BootstrapDataSecretInvalidReason, "Bootstrap secret value key is missing")
		return errors.New("error retrieving bootstrap data: secret value key is missing")
	}

	// Check if bootstrap data has changed and needs to be re-uploaded to S3
	bootstrapUserDataHash := fmt.Sprintf("%x", sha256.Sum256(bootstrapSecretValue))
	previousHash, annotationHashExists := karpenterMachinePool.Annotations[BootstrapDataHashAnnotation]
	if !annotationHashExists || previousHash != bootstrapUserDataHash {
		s3Client, err := r.awsClients.NewS3Client(awsCluster.Spec.Region, roleIdentity.Spec.RoleArn)
		if err != nil {
			return err
		}

		key := path.Join(S3ObjectPrefix, karpenterMachinePool.Name)

		logger.Info("Writing userdata to S3", "bucket", awsCluster.Spec.S3Bucket.Name, "key", key)
		if err = s3Client.Put(ctx, awsCluster.Spec.S3Bucket.Name, key, bootstrapSecretValue); err != nil {
			return err
		}

		// Create copy for patching annotations
		karpenterMachinePoolCopy := karpenterMachinePool.DeepCopy()

		// Update the hash annotation to track the current bootstrap data version
		if karpenterMachinePool.Annotations == nil {
			karpenterMachinePool.Annotations = make(map[string]string)
		}
		karpenterMachinePool.Annotations[BootstrapDataHashAnnotation] = bootstrapUserDataHash

		if err := r.client.Patch(ctx, karpenterMachinePool, client.MergeFrom(karpenterMachinePoolCopy)); err != nil {
			logger.Error(err, "failed to patch karpenterMachinePool.annotations with user data hash", "annotation", BootstrapDataHashAnnotation)
			return err
		}
	}

	return nil
}

// reconcileDelete deletes the karpenter custom resources from the workload cluster.
// When the cluster itself is being deleted, it also terminates all EC2 instances
// created by Karpenter to prevent orphaned resources.
func (r *KarpenterMachinePoolReconciler) reconcileDelete(ctx context.Context, logger logr.Logger, cluster *capi.Cluster, awsCluster *capa.AWSCluster, karpenterMachinePool *v1alpha1.KarpenterMachinePool, roleIdentity *capa.AWSClusterRoleIdentity) (reconcile.Result, error) {
	// We check if the owner Cluster is also being deleted (on top of the `KarpenterMachinePool` being deleted).
	// If the Cluster is being deleted, we terminate all the ec2 instances that karpenter may have launched.
	// These are normally removed by Karpenter, but when deleting a cluster, karpenter may not have enough time to clean them up.
	if !cluster.GetDeletionTimestamp().IsZero() {
		ec2Client, err := r.awsClients.NewEC2Client(awsCluster.Spec.Region, roleIdentity.Spec.RoleArn)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to create EC2 client: %w", err)
		}

		// Terminate EC2 instances with the karpenter.sh/nodepool tag matching the KarpenterMachinePool name
		instanceIDs, err := ec2Client.TerminateInstancesByTag(ctx, logger, "karpenter.sh/nodepool", karpenterMachinePool.Name)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to terminate EC2 instances: %w", err)
		}

		// Requeue if we find instances to terminate. On the next reconciliation, once there are no instances to terminate, we proceed to remove the finalizer.
		// We don't want to remove the finalizer when there may be ec2 instances still around to be cleaned up.
		if len(instanceIDs) > 0 {
			return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
		}
	}

	// Delete Karpenter resources from the workload cluster
	if err := r.deleteKarpenterResources(ctx, logger, cluster, karpenterMachinePool); err != nil {
		logger.Error(err, "failed to delete Karpenter resources")
		return reconcile.Result{}, err
	}

	// Create a deep copy of the reconciled object so we can change it
	karpenterMachinePoolCopy := karpenterMachinePool.DeepCopy()

	logger.Info("Removing finalizer", "finalizer", KarpenterFinalizer)
	controllerutil.RemoveFinalizer(karpenterMachinePool, KarpenterFinalizer)
	if err := r.client.Patch(ctx, karpenterMachinePool, client.MergeFrom(karpenterMachinePoolCopy)); err != nil {
		logger.Error(err, "failed to remove finalizer", "finalizer", KarpenterFinalizer)
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

// getWorkloadClusterNodeClaims retrieves all NodeClaim resources from the workload cluster.
// NodeClaims represent actual compute resources provisioned by Karpenter.
func (r *KarpenterMachinePoolReconciler) getWorkloadClusterNodeClaims(ctx context.Context, cluster *capi.Cluster) (*unstructured.UnstructuredList, error) {
	nodeClaimList := &unstructured.UnstructuredList{}
	workloadClusterClient, err := r.clusterClientGetter(ctx, "", r.client, client.ObjectKeyFromObject(cluster))
	if err != nil {
		return nodeClaimList, err
	}

	nodeClaimGVR := schema.GroupVersionResource{
		Group:    "karpenter.sh",
		Version:  "v1",
		Resource: "nodeclaims",
	}
	nodeClaimList.SetGroupVersionKind(nodeClaimGVR.GroupVersion().WithKind("NodeClaimList"))

	err = workloadClusterClient.List(ctx, nodeClaimList)
	return nodeClaimList, err
}

// computeProviderIDListFromNodeClaimsInWorkloadCluster extracts provider IDs from NodeClaims
// and returns both the list of provider IDs and the total count of node claims.
// Provider IDs are AWS-specific identifiers like "aws:///us-west-2a/i-1234567890abcdef0"
func (r *KarpenterMachinePoolReconciler) computeProviderIDListFromNodeClaimsInWorkloadCluster(ctx context.Context, logger logr.Logger, cluster *capi.Cluster) ([]string, int32, error) {
	var providerIDList []string

	nodeClaimList, err := r.getWorkloadClusterNodeClaims(ctx, cluster)
	if err != nil {
		return providerIDList, 0, err
	}

	for _, nc := range nodeClaimList.Items {
		providerID, found, err := unstructured.NestedString(nc.Object, "status", "providerID")
		if err != nil {
			logger.Error(err, "error retrieving nodeClaim.status.providerID", "nodeClaim", nc.GetName())
			continue
		}
		logger.Info("nodeClaim.status.providerID", "nodeClaimName", nc.GetName(), "statusFieldFound", found, "nodeClaim", nc.Object)
		if found && providerID != "" {
			providerIDList = append(providerIDList, providerID)
		}
	}

	// #nosec G115 -- len(nodeClaimList.Items) is guaranteed to be small in this context.
	return providerIDList, int32(len(nodeClaimList.Items)), nil
}

// getControlPlaneVersion retrieves the current Kubernetes version from the control plane.
// This is used for version skew validation to ensure workers don't run newer versions
// than the control plane, as defined in the version skew policy https://kubernetes.io/releases/version-skew-policy/.
func (r *KarpenterMachinePoolReconciler) getControlPlaneVersion(ctx context.Context, cluster *capi.Cluster) (string, error) {
	if cluster.Spec.ControlPlaneRef == nil {
		return "", fmt.Errorf("cluster has no control plane reference")
	}

	groupVersionKind := schema.GroupVersionKind{
		Group:   cluster.Spec.ControlPlaneRef.GroupVersionKind().Group,
		Version: cluster.Spec.ControlPlaneRef.GroupVersionKind().Version,
		Kind:    cluster.Spec.ControlPlaneRef.GroupVersionKind().Kind,
	}
	controlPlane := &unstructured.Unstructured{}
	controlPlane.SetGroupVersionKind(groupVersionKind)
	controlPlane.SetName(cluster.Spec.ControlPlaneRef.Name)
	controlPlane.SetNamespace(cluster.Spec.ControlPlaneRef.Namespace)

	if err := r.client.Get(ctx, client.ObjectKey{Name: cluster.Spec.ControlPlaneRef.Name, Namespace: cluster.Spec.ControlPlaneRef.Namespace}, controlPlane); err != nil {
		return "", fmt.Errorf("failed to get control plane %s: %w", cluster.Spec.ControlPlaneRef.Kind, err)
	}

	version, found, err := unstructured.NestedString(controlPlane.Object, "status", "version")
	if err != nil {
		return "", fmt.Errorf("failed to get current k8s version from control plane: %w", err)
	}
	if !found {
		return "", fmt.Errorf("version not found in control plane spec")
	}

	return version, nil
}

// createOrUpdateKarpenterResources creates or updates the Karpenter NodePool and EC2NodeClass custom resources in the workload cluster.
// This method enforces version skew policies and sets appropriate conditions based on success/failure states.
func (r *KarpenterMachinePoolReconciler) createOrUpdateKarpenterResources(ctx context.Context, logger logr.Logger, cluster *capi.Cluster, awsCluster *capa.AWSCluster, karpenterMachinePool *v1alpha1.KarpenterMachinePool, machinePool *capiexp.MachinePool) error {
	// Validate version skew: ensure worker nodes don't use newer Kubernetes versions than control plane
	allowed, controlPlaneCurrentVersion, nodePoolDesiredVersion, err := r.IsVersionSkewAllowed(ctx, cluster, machinePool)
	if err != nil {
		return err
	}

	if !allowed {
		message := fmt.Sprintf("Version skew policy violation: control plane version %s is older than node pool version %s", controlPlaneCurrentVersion, nodePoolDesiredVersion)
		logger.Info("Blocking Karpenter custom resources update due to version skew policy",
			"controlPlaneCurrentVersion", controlPlaneCurrentVersion,
			"nodePoolDesiredVersion", nodePoolDesiredVersion,
			"reason", message)

		// Mark version skew as invalid and resources as not ready
		conditions.MarkVersionSkewInvalid(karpenterMachinePool, conditions.VersionSkewBlockedReason, message)
		conditions.MarkEC2NodeClassNotCreated(karpenterMachinePool, conditions.VersionSkewBlockedReason, message)
		conditions.MarkNodePoolNotCreated(karpenterMachinePool, conditions.VersionSkewBlockedReason, message)

		return fmt.Errorf("version skew policy violation: %s", message)
	}

	// Mark version skew as valid
	conditions.MarkVersionSkewPolicySatisfied(karpenterMachinePool)

	// Persist version skew success condition immediately
	if statusErr := r.client.Status().Update(ctx, karpenterMachinePool); statusErr != nil {
		logger.Error(statusErr, "failed to update karpenterMachinePool status with version skew success condition")
		return fmt.Errorf("failed to persist version skew success condition: %w", statusErr)
	}

	workloadClusterClient, err := r.clusterClientGetter(ctx, "", r.client, client.ObjectKeyFromObject(cluster))
	if err != nil {
		return fmt.Errorf("failed to get workload cluster client: %w", err)
	}

	// Create or update EC2NodeClass
	if err := r.createOrUpdateEC2NodeClass(ctx, logger, workloadClusterClient, awsCluster, karpenterMachinePool); err != nil {
		conditions.MarkEC2NodeClassNotCreated(karpenterMachinePool, conditions.EC2NodeClassCreationFailedReason, fmt.Sprintf("%v", err))
		return fmt.Errorf("failed to create or update EC2NodeClass: %w", err)
	}
	conditions.MarkEC2NodeClassCreated(karpenterMachinePool)

	// Persist EC2NodeClass success condition immediately
	if statusErr := r.client.Status().Update(ctx, karpenterMachinePool); statusErr != nil {
		logger.Error(statusErr, "failed to update karpenterMachinePool status with EC2NodeClass success condition")
		return fmt.Errorf("failed to persist EC2NodeClass success condition: %w", statusErr)
	}

	// Create or update NodePool
	if err := r.createOrUpdateNodePool(ctx, logger, workloadClusterClient, cluster, karpenterMachinePool); err != nil {
		conditions.MarkNodePoolNotCreated(karpenterMachinePool, conditions.NodePoolCreationFailedReason, fmt.Sprintf("%v", err))
		return fmt.Errorf("failed to create or update NodePool: %w", err)
	}
	conditions.MarkNodePoolCreated(karpenterMachinePool)

	// Persist NodePool success condition immediately
	if statusErr := r.client.Status().Update(ctx, karpenterMachinePool); statusErr != nil {
		logger.Error(statusErr, "failed to update karpenterMachinePool status with NodePool success condition")
		return fmt.Errorf("failed to persist NodePool success condition: %w", statusErr)
	}

	return nil
}

// createOrUpdateEC2NodeClass creates or updates the EC2NodeClass resource in the workload cluster
// EC2NodeClass defines the EC2-specific configuration for nodes that Karpenter will provision,
// including AMI selection, instance profiles, security groups, and user data.
func (r *KarpenterMachinePoolReconciler) createOrUpdateEC2NodeClass(ctx context.Context, logger logr.Logger, workloadClusterClient client.Client, awsCluster *capa.AWSCluster, karpenterMachinePool *v1alpha1.KarpenterMachinePool) error {
	ec2NodeClassGVR := schema.GroupVersionResource{
		Group:    EC2NodeClassAPIGroup,
		Version:  "v1",
		Resource: "ec2nodeclasses",
	}

	ec2NodeClass := &unstructured.Unstructured{}
	ec2NodeClass.SetGroupVersionKind(ec2NodeClassGVR.GroupVersion().WithKind("EC2NodeClass"))
	ec2NodeClass.SetName(karpenterMachinePool.Name)
	ec2NodeClass.SetNamespace("")
	ec2NodeClass.SetLabels(map[string]string{"app.kubernetes.io/managed-by": "aws-resolver-rules-operator"})

	// Generate user data for Ignition
	userData := r.generateUserData(awsCluster.Spec.S3Bucket.Name, karpenterMachinePool.Name)

	operation, err := controllerutil.CreateOrUpdate(ctx, workloadClusterClient, ec2NodeClass, func() error {
		// Build the EC2NodeClass spec
		spec := map[string]interface{}{
			"amiFamily":           "Custom",
			"amiSelectorTerms":    karpenterMachinePool.Spec.EC2NodeClass.AMISelectorTerms,
			"blockDeviceMappings": karpenterMachinePool.Spec.EC2NodeClass.BlockDeviceMappings,
			"instanceProfile":     karpenterMachinePool.Spec.EC2NodeClass.InstanceProfile,
			"metadataOptions": map[string]interface{}{
				"httpEndpoint":            "enabled",
				"httpProtocolIPv6":        "disabled",
				"httpPutResponseHopLimit": 1,
				"httpTokens":              "required",
			},
			"securityGroupSelectorTerms": karpenterMachinePool.Spec.EC2NodeClass.SecurityGroupSelectorTerms,
			"subnetSelectorTerms":        karpenterMachinePool.Spec.EC2NodeClass.SubnetSelectorTerms,
			"userData":                   userData,
			"tags":                       mergeMaps(awsCluster.Spec.AdditionalTags, karpenterMachinePool.Spec.EC2NodeClass.Tags),
		}

		ec2NodeClass.Object["spec"] = spec
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to create or update EC2NodeClass: %w", err)
	}

	switch operation {
	case controllerutil.OperationResultCreated:
		logger.Info("Created EC2NodeClass")
	case controllerutil.OperationResultUpdated:
		logger.Info("Updated EC2NodeClass")
	}

	return nil
}

// mergeMaps combines multiple maps, with later maps taking precedence for duplicate keys
func mergeMaps[A comparable, B any](maps ...map[A]B) map[A]B {
	result := make(map[A]B)
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}

// createOrUpdateNodePool creates or updates the NodePool resource in the workload cluster.
// NodePool defines the desired state and constraints for nodes that Karpenter should provision,
// including resource limits, disruption policies, and node requirements.
func (r *KarpenterMachinePoolReconciler) createOrUpdateNodePool(ctx context.Context, logger logr.Logger, workloadClusterClient client.Client, cluster *capi.Cluster, karpenterMachinePool *v1alpha1.KarpenterMachinePool) error {
	nodePoolGVR := schema.GroupVersionResource{
		Group:    "karpenter.sh",
		Version:  "v1",
		Resource: "nodepools",
	}

	nodePool := &unstructured.Unstructured{}
	nodePool.SetGroupVersionKind(nodePoolGVR.GroupVersion().WithKind("NodePool"))
	nodePool.SetName(karpenterMachinePool.Name)
	nodePool.SetNamespace("")
	nodePool.SetLabels(map[string]string{"app.kubernetes.io/managed-by": "aws-resolver-rules-operator"})

	operation, err := controllerutil.CreateOrUpdate(ctx, workloadClusterClient, nodePool, func() error {
		spec := map[string]interface{}{
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{},
				"spec": map[string]interface{}{
					"startupTaints": []interface{}{
						map[string]interface{}{
							"effect": "NoExecute",
							"key":    "node.cilium.io/agent-not-ready",
							"value":  "true",
						},
						map[string]interface{}{
							"effect": "NoExecute",
							"key":    "node.cluster.x-k8s.io/uninitialized",
							"value":  "true",
						},
					},
					"nodeClassRef": map[string]interface{}{
						"group": EC2NodeClassAPIGroup,
						"kind":  "EC2NodeClass",
						"name":  karpenterMachinePool.Name,
					},
				},
			},
			"disruption": map[string]interface{}{},
		}

		// Apply user-defined NodePool configuration if provided
		if karpenterMachinePool.Spec.NodePool != nil {
			dis := spec["disruption"].(map[string]interface{})
			dis["budgets"] = karpenterMachinePool.Spec.NodePool.Disruption.Budgets
			dis["consolidateAfter"] = karpenterMachinePool.Spec.NodePool.Disruption.ConsolidateAfter
			dis["consolidationPolicy"] = karpenterMachinePool.Spec.NodePool.Disruption.ConsolidationPolicy

			if karpenterMachinePool.Spec.NodePool.Limits != nil {
				spec["limits"] = karpenterMachinePool.Spec.NodePool.Limits
			}

			if karpenterMachinePool.Spec.NodePool.Weight != nil {
				spec["weight"] = *karpenterMachinePool.Spec.NodePool.Weight
			}

			templateMetadata := spec["template"].(map[string]interface{})["metadata"].(map[string]interface{})
			templateMetadata["labels"] = karpenterMachinePool.Spec.NodePool.Template.ObjectMeta.Labels

			templateSpec := spec["template"].(map[string]interface{})["spec"].(map[string]interface{})

			templateSpec["taints"] = karpenterMachinePool.Spec.NodePool.Template.Spec.Taints
			templateSpec["requirements"] = karpenterMachinePool.Spec.NodePool.Template.Spec.Requirements
			templateSpec["expireAfter"] = karpenterMachinePool.Spec.NodePool.Template.Spec.ExpireAfter

			if karpenterMachinePool.Spec.NodePool.Template.Spec.TerminationGracePeriod != nil {
				templateSpec["terminationGracePeriod"] = karpenterMachinePool.Spec.NodePool.Template.Spec.TerminationGracePeriod
			}
		}

		nodePool.Object["spec"] = spec

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to create or update NodePool: %w", err)
	}

	switch operation {
	case controllerutil.OperationResultCreated:
		logger.Info("Created NodePool")
	case controllerutil.OperationResultUpdated:
		logger.Info("Updated NodePool")
	}

	return nil
}

// deleteKarpenterResources deletes the Karpenter NodePool and EC2NodeClass resources from the workload cluster.
func (r *KarpenterMachinePoolReconciler) deleteKarpenterResources(ctx context.Context, logger logr.Logger, cluster *capi.Cluster, karpenterMachinePool *v1alpha1.KarpenterMachinePool) error {
	workloadClusterClient, err := r.clusterClientGetter(ctx, "", r.client, client.ObjectKeyFromObject(cluster))
	if err != nil {
		return fmt.Errorf("failed to get workload cluster client: %w", err)
	}

	// Delete NodePool
	nodePoolGVR := schema.GroupVersionResource{
		Group:    "karpenter.sh",
		Version:  "v1",
		Resource: "nodepools",
	}

	nodePool := &unstructured.Unstructured{}
	nodePool.SetGroupVersionKind(nodePoolGVR.GroupVersion().WithKind("NodePool"))
	nodePool.SetName(karpenterMachinePool.Name)
	nodePool.SetNamespace("default")

	if err := workloadClusterClient.Delete(ctx, nodePool); err != nil && !k8serrors.IsNotFound(err) && !meta.IsNoMatchError(err) {
		logger.Error(err, "failed to delete NodePool", "name", karpenterMachinePool.Name)
		return fmt.Errorf("failed to delete NodePool: %w", err)
	}

	// Delete EC2NodeClass
	ec2NodeClassGVR := schema.GroupVersionResource{
		Group:    EC2NodeClassAPIGroup,
		Version:  "v1",
		Resource: "ec2nodeclasses",
	}

	ec2NodeClass := &unstructured.Unstructured{}
	ec2NodeClass.SetGroupVersionKind(ec2NodeClassGVR.GroupVersion().WithKind("EC2NodeClass"))
	ec2NodeClass.SetName(karpenterMachinePool.Name)
	ec2NodeClass.SetNamespace("default")

	if err := workloadClusterClient.Delete(ctx, ec2NodeClass); err != nil && !k8serrors.IsNotFound(err) && !meta.IsNoMatchError(err) {
		logger.Error(err, "failed to delete EC2NodeClass", "name", karpenterMachinePool.Name)
		return fmt.Errorf("failed to delete EC2NodeClass: %w", err)
	}

	return nil
}

// generateUserData generates the user data for Ignition configuration.
func (r *KarpenterMachinePoolReconciler) generateUserData(s3bucketName, karpenterMachinePoolName string) string {
	userData := map[string]interface{}{
		"ignition": map[string]interface{}{
			"config": map[string]interface{}{
				"merge": []map[string]interface{}{
					{
						"source":       fmt.Sprintf("s3://%s/%s/%s", s3bucketName, S3ObjectPrefix, karpenterMachinePoolName),
						"verification": map[string]interface{}{},
					},
				},
				"replace": map[string]interface{}{
					"verification": map[string]interface{}{},
				},
			},
			"proxy": map[string]interface{}{},
			"security": map[string]interface{}{
				"tls": map[string]interface{}{},
			},
			"timeouts": map[string]interface{}{},
			"version":  "3.4.0",
		},
		"kernelArguments": map[string]interface{}{},
		"passwd":          map[string]interface{}{},
		"storage":         map[string]interface{}{},
		"systemd":         map[string]interface{}{},
	}

	userDataBytes, _ := json.Marshal(userData)
	return string(userDataBytes)
}

// SetupWithManager sets up the controller with the Manager.
func (r *KarpenterMachinePoolReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	logger := capalogger.FromContext(ctx).GetLogger()

	return ctrl.NewControllerManagedBy(mgr).
		Named("karpentermachinepool").
		For(&v1alpha1.KarpenterMachinePool{}).
		WithEventFilter(predicates.ResourceNotPaused(logger)).
		Complete(r)
}

// IsVersionSkewAllowed checks if the worker version can be updated based on the control plane version.
// The workers can't use a newer k8s version than the one used by the control plane.
//
// This implements Kubernetes version skew policy https://kubernetes.io/releases/version-skew-policy/
//
// Returns: (allowed bool, controlPlaneVersion string, desiredWorkerVersion string, error)
func (r *KarpenterMachinePoolReconciler) IsVersionSkewAllowed(ctx context.Context, cluster *capi.Cluster, machinePool *capiexp.MachinePool) (bool, string, string, error) {
	controlPlaneVersion, err := r.getControlPlaneVersion(ctx, cluster)
	if err != nil {
		return true, "", "", fmt.Errorf("failed to get current Control Plane k8s version: %w", err)
	}

	// Parse versions using semantic versioning for proper comparison
	controlPlaneCurrentK8sVersion, err := semver.ParseTolerant(controlPlaneVersion)
	if err != nil {
		return true, "", "", fmt.Errorf("failed to parse current Control Plane k8s version: %w", err)
	}

	machinePoolDesiredK8sVersion, err := semver.ParseTolerant(*machinePool.Spec.Template.Spec.Version)
	if err != nil {
		return true, controlPlaneVersion, "", fmt.Errorf("failed to parse node pool desired k8s version: %w", err)
	}

	// Allow if control plane version >= desired worker version
	return controlPlaneCurrentK8sVersion.GE(machinePoolDesiredK8sVersion), controlPlaneVersion, *machinePool.Spec.Template.Spec.Version, nil
}
