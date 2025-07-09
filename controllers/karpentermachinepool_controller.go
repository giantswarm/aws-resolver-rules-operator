package controllers

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
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
	KarpenterFinalizer          = "capa-operator.finalizers.giantswarm.io/karpenter-controller"
	S3ObjectPrefix              = "karpenter-machine-pool"
	// KarpenterNodePoolReadyCondition reports on current status of the autoscaling group. Ready indicates the group is provisioned.
	KarpenterNodePoolReadyCondition capi.ConditionType = "KarpenterNodePoolReadyCondition"
	// WaitingForBootstrapDataReason used when machine is waiting for bootstrap data to be ready before proceeding.
	WaitingForBootstrapDataReason = "WaitingForBootstrapData"
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

// Reconcile will upload to S3 the Ignition configuration for the reconciled node pool.
// It will also take care of deleting EC2 instances created by karpenter when the cluster is being removed.
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

	// Create deep copy of the reconciled object so we can change it
	karpenterMachinePoolCopy := karpenterMachinePool.DeepCopy()

	// We only remove the finalizer after we've removed the EC2 instances created by karpenter.
	// These are normally removed by Karpenter but when deleting a cluster, karpenter may not have enough time to clean them up.
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

	updated := controllerutil.AddFinalizer(karpenterMachinePool, KarpenterFinalizer)
	if updated {
		if err := r.client.Patch(ctx, karpenterMachinePool, client.MergeFrom(karpenterMachinePoolCopy)); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer to KarpenterMachinePool: %w", err)
		}
	}

	// Create or update Karpenter resources in the workload cluster
	if err := r.createOrUpdateKarpenterResources(ctx, logger, cluster, awsCluster, karpenterMachinePool, machinePool, bootstrapSecretValue); err != nil {
		logger.Error(err, "failed to create or update Karpenter resources")
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

	if numberOfNodeClaims == 0 {
		// Karpenter has not reacted yet, let's requeue
		return reconcile.Result{RequeueAfter: 1 * time.Minute}, nil
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

func (r *KarpenterMachinePoolReconciler) reconcileDelete(ctx context.Context, logger logr.Logger, cluster *capi.Cluster, awsCluster *capa.AWSCluster, karpenterMachinePool *v1alpha1.KarpenterMachinePool, roleIdentity *capa.AWSClusterRoleIdentity) (reconcile.Result, error) {
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

		// Requeue if we find instances to terminate. Once there are no instances to terminate, we proceed to remove the finalizer.
		// We do this when the cluster is being deleted, to avoid removing the finalizer before karpenter launches a new instance that would be left over.
		if len(instanceIDs) > 0 {
			return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
		}
	}

	// Delete Karpenter resources from the workload cluster
	if err := r.deleteKarpenterResources(ctx, logger, cluster, karpenterMachinePool); err != nil {
		logger.Error(err, "failed to delete Karpenter resources")
		return reconcile.Result{}, err
	}

	// Create deep copy of the reconciled object so we can change it
	karpenterMachinePoolCopy := karpenterMachinePool.DeepCopy()

	logger.Info("Removing finalizer", "finalizer", KarpenterFinalizer)
	controllerutil.RemoveFinalizer(karpenterMachinePool, KarpenterFinalizer)
	if err := r.client.Patch(ctx, karpenterMachinePool, client.MergeFrom(karpenterMachinePoolCopy)); err != nil {
		logger.Error(err, "failed to remove finalizer", "finalizer", KarpenterFinalizer)
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func getWorkloadClusterNodeClaims(ctx context.Context, ctrlClient client.Client) (*unstructured.UnstructuredList, error) {
	nodeClaimGVR := schema.GroupVersionResource{
		Group:    "karpenter.sh",
		Version:  "v1",
		Resource: "nodeclaims",
	}
	nodeClaimList := &unstructured.UnstructuredList{}
	nodeClaimList.SetGroupVersionKind(nodeClaimGVR.GroupVersion().WithKind("NodeClaimList"))

	err := ctrlClient.List(ctx, nodeClaimList)
	return nodeClaimList, err
}

func (r *KarpenterMachinePoolReconciler) computeProviderIDListFromNodeClaimsInWorkloadCluster(ctx context.Context, logger logr.Logger, cluster *capi.Cluster) ([]string, int32, error) {
	var providerIDList []string

	workloadClusterClient, err := r.clusterClientGetter(ctx, "", r.client, client.ObjectKeyFromObject(cluster))
	if err != nil {
		return providerIDList, 0, err
	}

	nodeClaimList, err := getWorkloadClusterNodeClaims(ctx, workloadClusterClient)
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

// getControlPlaneVersion retrieves the Kubernetes version from the control plane
func (r *KarpenterMachinePoolReconciler) getControlPlaneVersion(ctx context.Context, cluster *capi.Cluster) (string, error) {
	if cluster.Spec.ControlPlaneRef == nil {
		return "", fmt.Errorf("cluster has no control plane reference")
	}

	// Parse the API version to get group and version
	apiVersion := cluster.Spec.ControlPlaneRef.APIVersion
	groupVersion, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return "", fmt.Errorf("failed to parse control plane API version %s: %w", apiVersion, err)
	}

	// Create the GVR using the parsed group and version
	controlPlaneGVR := schema.GroupVersionResource{
		Group:    groupVersion.Group,
		Version:  groupVersion.Version,
		Resource: strings.ToLower(cluster.Spec.ControlPlaneRef.Kind) + "s", // Convert Kind to resource name
	}

	controlPlane := &unstructured.Unstructured{}
	controlPlane.SetGroupVersionKind(controlPlaneGVR.GroupVersion().WithKind(cluster.Spec.ControlPlaneRef.Kind))
	controlPlane.SetName(cluster.Spec.ControlPlaneRef.Name)
	controlPlane.SetNamespace(cluster.Spec.ControlPlaneRef.Namespace)

	if err := r.client.Get(ctx, client.ObjectKey{Name: cluster.Spec.ControlPlaneRef.Name, Namespace: cluster.Spec.ControlPlaneRef.Namespace}, controlPlane); err != nil {
		return "", fmt.Errorf("failed to get control plane %s: %w", cluster.Spec.ControlPlaneRef.Kind, err)
	}

	version, found, err := unstructured.NestedString(controlPlane.Object, "spec", "version")
	if err != nil {
		return "", fmt.Errorf("failed to get version from control plane: %w", err)
	}
	if !found {
		return "", fmt.Errorf("version not found in control plane spec")
	}

	return version, nil
}

// createOrUpdateKarpenterResources creates or updates the Karpenter NodePool and EC2NodeClass resources in the workload cluster
func (r *KarpenterMachinePoolReconciler) createOrUpdateKarpenterResources(ctx context.Context, logger logr.Logger, cluster *capi.Cluster, awsCluster *capa.AWSCluster, karpenterMachinePool *v1alpha1.KarpenterMachinePool, machinePool *capiexp.MachinePool, bootstrapSecretValue []byte) error {
	// Get the worker version from MachinePool
	workerVersion := ""
	if machinePool.Spec.Template.Spec.Version != nil {
		workerVersion = *machinePool.Spec.Template.Spec.Version
	}

	// Get control plane version and check version skew
	if workerVersion != "" {
		controlPlaneVersion, err := r.getControlPlaneVersion(ctx, cluster)
		if err != nil {
			logger.Error(err, "Failed to get control plane version, proceeding with update")
		} else {
			allowed, err := IsVersionSkewAllowed(controlPlaneVersion, workerVersion)
			if err != nil {
				logger.Error(err, "Failed to check version skew, proceeding with update")
			} else if !allowed {
				message := fmt.Sprintf("Version skew policy violation: control plane version %s is more than 2 minor versions ahead of worker version %s", controlPlaneVersion, workerVersion)
				logger.Info("Blocking Karpenter resource update due to version skew policy",
					"controlPlaneVersion", controlPlaneVersion,
					"workerVersion", workerVersion,
					"reason", message)

				// Mark resources as not ready due to version skew
				conditions.MarkEC2NodeClassNotReady(karpenterMachinePool, VersionSkewBlockedReason, message)
				conditions.MarkNodePoolNotReady(karpenterMachinePool, VersionSkewBlockedReason, message)

				return fmt.Errorf("version skew policy violation: %s", message)
			}
		}
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
	if err := r.createOrUpdateNodePool(ctx, logger, workloadClusterClient, karpenterMachinePool); err != nil {
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
		Group:    "karpenter.k8s.aws",
		Version:  "v1",
		Resource: "ec2nodeclasses",
	}

	ec2NodeClass := &unstructured.Unstructured{}
	ec2NodeClass.SetGroupVersionKind(ec2NodeClassGVR.GroupVersion().WithKind("EC2NodeClass"))
	ec2NodeClass.SetName(karpenterMachinePool.Name)
	ec2NodeClass.SetNamespace("default")

	// Generate user data for Ignition
	userData := r.generateUserData(awsCluster.Spec.S3Bucket.Name, cluster.Name, karpenterMachinePool.Name)

	// Add security groups tag selector if specified
	securityGroupTagsSelector := map[string]string{
		fmt.Sprintf("sigs.k8s.io/cluster-api-provider-aws/cluster/%s", cluster.Name): "owned",
		"sigs.k8s.io/cluster-api-provider-aws/role":                                  "node",
	}
	if karpenterMachinePool.Spec.EC2NodeClass != nil && len(karpenterMachinePool.Spec.EC2NodeClass.SecurityGroups) > 0 {
		for securityGroupTagKey, securityGroupTagValue := range karpenterMachinePool.Spec.EC2NodeClass.SecurityGroups {
			securityGroupTagsSelector[securityGroupTagKey] = securityGroupTagValue
		}
	}

	// Add subnet tag selector if specified
	subnetTagsSelector := map[string]string{
		fmt.Sprintf("sigs.k8s.io/cluster-api-provider-aws/cluster/%s", cluster.Name): "owned",
		"giantswarm.io/role": "nodes",
	}
	if karpenterMachinePool.Spec.EC2NodeClass != nil && len(karpenterMachinePool.Spec.EC2NodeClass.Subnets) > 0 {
		for subnetTagKey, subnetTagValue := range karpenterMachinePool.Spec.EC2NodeClass.Subnets {
			subnetTagsSelector[subnetTagKey] = subnetTagValue
		}
	}

	operation, err := controllerutil.CreateOrUpdate(ctx, workloadClusterClient, ec2NodeClass, func() error {
		// Build the EC2NodeClass spec
		spec := map[string]interface{}{
			"amiFamily": "AL2",
			"amiSelectorTerms": []map[string]interface{}{
				{
					"name":  karpenterMachinePool.Spec.EC2NodeClass.AMIName,
					"owner": karpenterMachinePool.Spec.EC2NodeClass.AMIOwner,
				},
			},
			"instanceProfile": karpenterMachinePool.Spec.IamInstanceProfile,
			"securityGroupSelectorTerms": []map[string]interface{}{
				{
					"tags": securityGroupTagsSelector,
				},
			},
			"subnetSelectorTerms": []map[string]interface{}{
				{
					"tags": subnetTagsSelector,
				},
			},
			"userData": userData,
		}

		// Add tags if specified
		if karpenterMachinePool.Spec.EC2NodeClass != nil && len(karpenterMachinePool.Spec.EC2NodeClass.Tags) > 0 {
			spec["tags"] = karpenterMachinePool.Spec.EC2NodeClass.Tags
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
func (r *KarpenterMachinePoolReconciler) createOrUpdateNodePool(ctx context.Context, logger logr.Logger, workloadClusterClient client.Client, karpenterMachinePool *v1alpha1.KarpenterMachinePool) error {
	nodePoolGVR := schema.GroupVersionResource{
		Group:    "karpenter.sh",
		Version:  "v1",
		Resource: "nodepools",
	}

	nodePool := &unstructured.Unstructured{}
	nodePool.SetGroupVersionKind(nodePoolGVR.GroupVersion().WithKind("NodePool"))
	nodePool.SetName(karpenterMachinePool.Name)
	nodePool.SetNamespace("default")

	operation, err := controllerutil.CreateOrUpdate(ctx, workloadClusterClient, nodePool, func() error {
		// Build the NodePool spec
		spec := map[string]interface{}{
			"disruption": map[string]interface{}{
				"consolidateAfter": "30s",
			},
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"nodeClassRef": map[string]interface{}{
						"apiVersion": "karpenter.k8s.aws/v1",
						"group":      "karpenter.k8s.aws",
						"kind":       "EC2NodeClass",
						"name":       karpenterMachinePool.Name,
					},
					"requirements": []interface{}{},
				},
			},
		}

		// Add NodePool configuration if specified
		if karpenterMachinePool.Spec.NodePool != nil {
			if karpenterMachinePool.Spec.NodePool.Disruption != nil {
				if karpenterMachinePool.Spec.NodePool.Disruption.ConsolidateAfter != nil {
					spec["disruption"].(map[string]interface{})["consolidateAfter"] = karpenterMachinePool.Spec.NodePool.Disruption.ConsolidateAfter.Duration.String()
				}
				if karpenterMachinePool.Spec.NodePool.Disruption.ConsolidationPolicy != nil {
					spec["disruption"].(map[string]interface{})["consolidationPolicy"] = *karpenterMachinePool.Spec.NodePool.Disruption.ConsolidationPolicy
				}
			}

			if karpenterMachinePool.Spec.NodePool.Limits != nil {
				limits := map[string]interface{}{}
				if karpenterMachinePool.Spec.NodePool.Limits.CPU != nil {
					limits["cpu"] = karpenterMachinePool.Spec.NodePool.Limits.CPU.String()
				}
				if karpenterMachinePool.Spec.NodePool.Limits.Memory != nil {
					limits["memory"] = karpenterMachinePool.Spec.NodePool.Limits.Memory.String()
				}
				if len(limits) > 0 {
					spec["limits"] = limits
				}
			}

			if len(karpenterMachinePool.Spec.NodePool.Requirements) > 0 {
				requirements := []map[string]interface{}{}
				for _, req := range karpenterMachinePool.Spec.NodePool.Requirements {
					requirement := map[string]interface{}{
						"key":      req.Key,
						"operator": req.Operator,
					}
					if len(req.Values) > 0 {
						requirement["values"] = req.Values
					}
					requirements = append(requirements, requirement)
				}

				spec["template"].(map[string]interface{})["spec"].(map[string]interface{})["requirements"] = requirements
			}

			if len(karpenterMachinePool.Spec.NodePool.Taints) > 0 {
				taints := []map[string]interface{}{}
				for _, taint := range karpenterMachinePool.Spec.NodePool.Taints {
					taintMap := map[string]interface{}{
						"key":    taint.Key,
						"effect": taint.Effect,
					}
					if taint.Value != nil {
						taintMap["value"] = *taint.Value
					}
					taints = append(taints, taintMap)
				}
				spec["taints"] = taints
			}

			if len(karpenterMachinePool.Spec.NodePool.Labels) > 0 {
				spec["labels"] = karpenterMachinePool.Spec.NodePool.Labels
			}

			if karpenterMachinePool.Spec.NodePool.Weight != nil {
				spec["weight"] = *karpenterMachinePool.Spec.NodePool.Weight
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

	if err := workloadClusterClient.Delete(ctx, nodePool); err != nil && !k8serrors.IsNotFound(err) {
		logger.Error(err, "failed to delete NodePool", "name", karpenterMachinePool.Name)
		return fmt.Errorf("failed to delete NodePool: %w", err)
	}

	// Delete EC2NodeClass
	ec2NodeClassGVR := schema.GroupVersionResource{
		Group:    "karpenter.k8s.aws",
		Version:  "v1",
		Resource: "ec2nodeclasses",
	}

	ec2NodeClass := &unstructured.Unstructured{}
	ec2NodeClass.SetGroupVersionKind(ec2NodeClassGVR.GroupVersion().WithKind("EC2NodeClass"))
	ec2NodeClass.SetName(karpenterMachinePool.Name)
	ec2NodeClass.SetNamespace("default")

	if err := workloadClusterClient.Delete(ctx, ec2NodeClass); err != nil && !k8serrors.IsNotFound(err) {
		logger.Error(err, "failed to delete EC2NodeClass", "name", karpenterMachinePool.Name)
		return fmt.Errorf("failed to delete EC2NodeClass: %w", err)
	}

	return nil
}

// generateUserData generates the user data for Ignition configuration
func (r *KarpenterMachinePoolReconciler) generateUserData(s3bucketName, clusterName, karpenterMachinePoolName string) string {
	userData := map[string]interface{}{
		"ignition": map[string]interface{}{
			"config": map[string]interface{}{
				"merge": []map[string]interface{}{
					{
						"source":       fmt.Sprintf("s3://%s/%s/%s-%s", s3bucketName, S3ObjectPrefix, clusterName, karpenterMachinePoolName),
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

// CompareKubernetesVersions compares two Kubernetes versions and returns:
// -1 if version1 < version2
//
//	0 if version1 == version2
//
// +1 if version1 > version2
func CompareKubernetesVersions(version1, version2 string) (int, error) {
	// Remove 'v' prefix if present
	v1 := strings.TrimPrefix(version1, "v")
	v2 := strings.TrimPrefix(version2, "v")

	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	if len(parts1) < 2 || len(parts2) < 2 {
		return 0, fmt.Errorf("invalid version format: %s or %s", version1, version2)
	}

	// Compare major version
	major1, err := strconv.Atoi(parts1[0])
	if err != nil {
		return 0, fmt.Errorf("invalid major version in %s: %w", version1, err)
	}
	major2, err := strconv.Atoi(parts2[0])
	if err != nil {
		return 0, fmt.Errorf("invalid major version in %s: %w", version2, err)
	}

	if major1 != major2 {
		if major1 < major2 {
			return -1, nil
		}
		return 1, nil
	}

	// Compare minor version
	minor1, err := strconv.Atoi(parts1[1])
	if err != nil {
		return 0, fmt.Errorf("invalid minor version in %s: %w", version1, err)
	}
	minor2, err := strconv.Atoi(parts2[1])
	if err != nil {
		return 0, fmt.Errorf("invalid minor version in %s: %w", version2, err)
	}

	if minor1 < minor2 {
		return -1, nil
	} else if minor1 > minor2 {
		return 1, nil
	}

	// If major and minor are the same, compare patch version if available
	if len(parts1) >= 3 && len(parts2) >= 3 {
		patch1, err := strconv.Atoi(parts1[2])
		if err != nil {
			return 0, fmt.Errorf("invalid patch version in %s: %w", version1, err)
		}
		patch2, err := strconv.Atoi(parts2[2])
		if err != nil {
			return 0, fmt.Errorf("invalid patch version in %s: %w", version2, err)
		}

		if patch1 < patch2 {
			return -1, nil
		} else if patch1 > patch2 {
			return 1, nil
		}
	}

	return 0, nil
}

// IsVersionSkewAllowed checks if the worker version can be updated based on the control plane version
// According to Kubernetes version skew policy, workers can be at most 2 minor versions behind the control plane
func IsVersionSkewAllowed(controlPlaneVersion, workerVersion string) (bool, error) {
	comparison, err := CompareKubernetesVersions(controlPlaneVersion, workerVersion)
	if err != nil {
		return false, err
	}

	// If control plane version is older than or equal to worker version, allow the update
	if comparison <= 0 {
		return true, nil
	}

	// Parse versions to check minor version difference
	v1 := strings.TrimPrefix(controlPlaneVersion, "v")
	v2 := strings.TrimPrefix(workerVersion, "v")

	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	if len(parts1) < 2 || len(parts2) < 2 {
		return false, fmt.Errorf("invalid version format: %s or %s", controlPlaneVersion, workerVersion)
	}

	minor1, err := strconv.Atoi(parts1[1])
	if err != nil {
		return false, fmt.Errorf("invalid minor version in %s: %w", controlPlaneVersion, err)
	}
	minor2, err := strconv.Atoi(parts2[1])
	if err != nil {
		return false, fmt.Errorf("invalid minor version in %s: %w", workerVersion, err)
	}

	// Allow if the difference is at most 2 minor versions
	versionDiff := minor1 - minor2
	return versionDiff <= 2, nil
}
