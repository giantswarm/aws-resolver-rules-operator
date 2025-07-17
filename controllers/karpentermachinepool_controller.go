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
	BootstrapDataHashAnnotation = "giantswarm.io/userdata-hash"
	EC2NodeClassAPIGroup        = "karpenter.k8s.aws"
	KarpenterFinalizer          = "capa-operator.finalizers.giantswarm.io/karpenter-controller"
	S3ObjectPrefix              = "karpenter-machine-pool"
	// NodePoolCreationFailedReason indicates that the NodePool creation failed
	NodePoolCreationFailedReason = "NodePoolCreationFailed"
	// EC2NodeClassCreationFailedReason indicates that the EC2NodeClass creation failed
	EC2NodeClassCreationFailedReason = "EC2NodeClassCreationFailed"
	// VersionSkewBlockedReason indicates that the update was blocked due to version skew policy
	VersionSkewBlockedReason = "VersionSkewBlocked"
)

type KarpenterMachinePoolReconciler struct {
	awsClients resolver.AWSClients
	client     client.Client
	// clusterClientGetter is used to create a client targeting the workload cluster
	clusterClientGetter remote.ClusterClientGetter
}

func NewKarpenterMachinepoolReconciler(client client.Client, clusterClientGetter remote.ClusterClientGetter, awsClients resolver.AWSClients) *KarpenterMachinePoolReconciler {
	return &KarpenterMachinePoolReconciler{awsClients: awsClients, client: client, clusterClientGetter: clusterClientGetter}
}

// Reconcile reconciles KarpenterMachinePool, which represent cluster node pools that will be managed by karpenter.
// The controller will upload to S3 the Ignition configuration for the reconciled node pool.
// It will also take care of deleting EC2 instances created by karpenter when the cluster is being removed.
// And lastly, it will create the karpenter CRs in the workload cluster.
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

	if awsCluster.Spec.S3Bucket == nil {
		return reconcile.Result{}, errors.New("a cluster wide object storage configured at `AWSCluster.spec.s3Bucket` is required")
	}

	roleIdentity := &capa.AWSClusterRoleIdentity{}
	if err = r.client.Get(ctx, client.ObjectKey{Name: awsCluster.Spec.IdentityRef.Name}, roleIdentity); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get AWSClusterRoleIdentity referenced in AWSCluster: %w", err)
	}

	if !karpenterMachinePool.GetDeletionTimestamp().IsZero() {
		return r.reconcileDelete(ctx, logger, cluster, awsCluster, karpenterMachinePool, roleIdentity)
	}

	bootstrapSecret := &v1.Secret{}
	if err := r.client.Get(ctx, client.ObjectKey{Namespace: req.Namespace, Name: *machinePool.Spec.Template.Spec.Bootstrap.DataSecretName}, bootstrapSecret); err != nil {
		return reconcile.Result{}, fmt.Errorf("bootstrap secret in MachinePool.spec.template.spec.bootstrap.dataSecretName is not found: %w", err)
	}

	bootstrapSecretValue, ok := bootstrapSecret.Data["value"]
	if !ok {
		return reconcile.Result{}, errors.New("error retrieving bootstrap data: secret value key is missing")
	}

	// Create a deep copy of the reconciled object so we can change it
	karpenterMachinePoolCopy := karpenterMachinePool.DeepCopy()
	updated := controllerutil.AddFinalizer(karpenterMachinePool, KarpenterFinalizer)
	if updated {
		if err := r.client.Patch(ctx, karpenterMachinePool, client.MergeFrom(karpenterMachinePoolCopy)); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer to KarpenterMachinePool: %w", err)
		}
	}

	// Create or update Karpenter custom resources in the workload cluster.
	if err := r.createOrUpdateKarpenterResources(ctx, logger, cluster, awsCluster, karpenterMachinePool, machinePool, bootstrapSecretValue); err != nil {
		logger.Error(err, "failed to create or update Karpenter custom resources in the workload cluster")
		return reconcile.Result{}, err
	}

	bootstrapUserDataHash := fmt.Sprintf("%x", sha256.Sum256(bootstrapSecretValue))
	previousHash, annotationHashExists := karpenterMachinePool.Annotations[BootstrapDataHashAnnotation]
	if !annotationHashExists || previousHash != bootstrapUserDataHash {
		s3Client, err := r.awsClients.NewS3Client(awsCluster.Spec.Region, roleIdentity.Spec.RoleArn)
		if err != nil {
			return reconcile.Result{}, err
		}

		key := path.Join(S3ObjectPrefix, req.Name)

		logger.Info("Writing userdata to S3", "bucket", awsCluster.Spec.S3Bucket.Name, "key", key)
		if err = s3Client.Put(ctx, awsCluster.Spec.S3Bucket.Name, key, bootstrapSecretValue); err != nil {
			return reconcile.Result{}, err
		}

		if karpenterMachinePool.Annotations == nil {
			karpenterMachinePool.Annotations = make(map[string]string)
		}
		karpenterMachinePool.Annotations[BootstrapDataHashAnnotation] = bootstrapUserDataHash

		if err := r.client.Patch(ctx, karpenterMachinePool, client.MergeFrom(karpenterMachinePoolCopy)); err != nil {
			logger.Error(err, "failed to patch karpenterMachinePool.annotations with user data hash", "annotation", BootstrapDataHashAnnotation)
			return reconcile.Result{}, err
		}
	}

	providerIDList, numberOfNodeClaims, err := r.computeProviderIDListFromNodeClaimsInWorkloadCluster(ctx, logger, cluster)
	if err != nil {
		return reconcile.Result{}, err
	}

	karpenterMachinePool.Status.Replicas = numberOfNodeClaims
	karpenterMachinePool.Status.Ready = true

	logger.Info("Found NodeClaims in workload cluster, patching KarpenterMachinePool", "numberOfNodeClaims", numberOfNodeClaims, "providerIDList", providerIDList)

	if err := r.client.Status().Patch(ctx, karpenterMachinePool, client.MergeFrom(karpenterMachinePoolCopy), client.FieldOwner("karpentermachinepool-controller")); err != nil {
		logger.Error(err, "failed to patch karpenterMachinePool.status.Replicas")
		return reconcile.Result{}, err
	}

	karpenterMachinePool.Spec.ProviderIDList = providerIDList

	if err := r.client.Patch(ctx, karpenterMachinePool, client.MergeFrom(karpenterMachinePoolCopy), client.FieldOwner("karpentermachinepool-controller")); err != nil {
		logger.Error(err, "failed to patch karpenterMachinePool.spec.providerIDList")
		return reconcile.Result{}, err
	}

	if machinePool.Spec.Replicas == nil || *machinePool.Spec.Replicas != numberOfNodeClaims {
		machinePoolCopy := machinePool.DeepCopy()
		machinePool.Spec.Replicas = &numberOfNodeClaims
		if err := r.client.Patch(ctx, machinePool, client.MergeFrom(machinePoolCopy), client.FieldOwner("karpenter-machinepool-controller")); err != nil {
			logger.Error(err, "failed to patch MachinePool.spec.replicas")
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

// reconcileDelete deletes the karpenter custom resources from the workload cluster.
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

// getControlPlaneVersion retrieves the current Kubernetes version from the control plane
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

// createOrUpdateKarpenterResources creates or updates the Karpenter NodePool and EC2NodeClass custom resources in the workload cluster
func (r *KarpenterMachinePoolReconciler) createOrUpdateKarpenterResources(ctx context.Context, logger logr.Logger, cluster *capi.Cluster, awsCluster *capa.AWSCluster, karpenterMachinePool *v1alpha1.KarpenterMachinePool, machinePool *capiexp.MachinePool, bootstrapSecretValue []byte) error {
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

		// Mark resources as not ready due to version skew
		conditions.MarkEC2NodeClassNotReady(karpenterMachinePool, VersionSkewBlockedReason, message)
		conditions.MarkNodePoolNotReady(karpenterMachinePool, VersionSkewBlockedReason, message)

		return fmt.Errorf("version skew policy violation: %s", message)
	}

	workloadClusterClient, err := r.clusterClientGetter(ctx, "", r.client, client.ObjectKeyFromObject(cluster))
	if err != nil {
		return fmt.Errorf("failed to get workload cluster client: %w", err)
	}

	// Create or update EC2NodeClass
	if err := r.createOrUpdateEC2NodeClass(ctx, logger, workloadClusterClient, cluster, awsCluster, karpenterMachinePool, bootstrapSecretValue); err != nil {
		conditions.MarkEC2NodeClassNotReady(karpenterMachinePool, EC2NodeClassCreationFailedReason, err.Error())
		return fmt.Errorf("failed to create or update EC2NodeClass: %w", err)
	}

	// Create or update NodePool
	if err := r.createOrUpdateNodePool(ctx, logger, workloadClusterClient, cluster, karpenterMachinePool); err != nil {
		conditions.MarkNodePoolNotReady(karpenterMachinePool, NodePoolCreationFailedReason, err.Error())
		return fmt.Errorf("failed to create or update NodePool: %w", err)
	}

	// Mark both resources as ready
	conditions.MarkEC2NodeClassReady(karpenterMachinePool)
	conditions.MarkNodePoolReady(karpenterMachinePool)

	return nil
}

// createOrUpdateEC2NodeClass creates or updates the EC2NodeClass resource in the workload cluster
func (r *KarpenterMachinePoolReconciler) createOrUpdateEC2NodeClass(ctx context.Context, logger logr.Logger, workloadClusterClient client.Client, cluster *capi.Cluster, awsCluster *capa.AWSCluster, karpenterMachinePool *v1alpha1.KarpenterMachinePool, bootstrapSecretValue []byte) error {
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

// createOrUpdateNodePool creates or updates the NodePool resource in the workload cluster
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

// deleteKarpenterResources deletes the Karpenter NodePool and EC2NodeClass resources from the workload cluster
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

// generateUserData generates the user data for Ignition configuration
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
func (r *KarpenterMachinePoolReconciler) IsVersionSkewAllowed(ctx context.Context, cluster *capi.Cluster, machinePool *capiexp.MachinePool) (bool, string, string, error) {
	controlPlaneVersion, err := r.getControlPlaneVersion(ctx, cluster)
	if err != nil {
		return true, "", "", fmt.Errorf("failed to get current Control Plane k8s version: %w", err)
	}

	controlPlaneCurrentK8sVersion, err := semver.ParseTolerant(controlPlaneVersion)
	if err != nil {
		return true, "", "", fmt.Errorf("failed to parse current Control Plane k8s version: %w", err)
	}

	machinePoolDesiredK8sVersion, err := semver.ParseTolerant(*machinePool.Spec.Template.Spec.Version)
	if err != nil {
		return true, controlPlaneVersion, "", fmt.Errorf("failed to parse node pool desired k8s version: %w", err)
	}

	return controlPlaneCurrentK8sVersion.GE(machinePoolDesiredK8sVersion), controlPlaneVersion, *machinePool.Spec.Template.Spec.Version, nil
}
