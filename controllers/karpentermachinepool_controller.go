package controllers

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"path"
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
	// NodePoolCreatedReason indicates that the NodePool was successfully created
	NodePoolCreatedReason = "NodePoolCreated"
	// NodePoolCreationFailedReason indicates that the NodePool creation failed
	NodePoolCreationFailedReason = "NodePoolCreationFailed"
	// EC2NodeClassCreatedReason indicates that the EC2NodeClass was successfully created
	EC2NodeClassCreatedReason = "EC2NodeClassCreated"
	// EC2NodeClassCreationFailedReason indicates that the EC2NodeClass creation failed
	EC2NodeClassCreationFailedReason = "EC2NodeClassCreationFailed"
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

	// Create or update Karpenter resources in the workload cluster
	if err := r.createOrUpdateKarpenterResources(ctx, logger, cluster, awsCluster, karpenterMachinePool, bootstrapSecretValue); err != nil {
		logger.Error(err, "failed to create or update Karpenter resources")
		return reconcile.Result{}, err
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

	logger.Info("Found NodeClaims in workload cluster, patching KarpenterMachinePool", "numberOfNodeClaims", numberOfNodeClaims)

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
		if found && providerID != "" {
			providerIDList = append(providerIDList, providerID)
		}
	}

	// #nosec G115 -- len(nodeClaimList.Items) is guaranteed to be small in this context.
	return providerIDList, int32(len(nodeClaimList.Items)), nil
}

// createOrUpdateKarpenterResources creates or updates the Karpenter NodePool and EC2NodeClass resources in the workload cluster
func (r *KarpenterMachinePoolReconciler) createOrUpdateKarpenterResources(ctx context.Context, logger logr.Logger, cluster *capi.Cluster, awsCluster *capa.AWSCluster, karpenterMachinePool *v1alpha1.KarpenterMachinePool, bootstrapSecretValue []byte) error {
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
		Version:  "v1beta1",
		Resource: "ec2nodeclasses",
	}

	ec2NodeClass := &unstructured.Unstructured{}
	ec2NodeClass.SetGroupVersionKind(ec2NodeClassGVR.GroupVersion().WithKind("EC2NodeClass"))
	ec2NodeClass.SetName(karpenterMachinePool.Name)
	ec2NodeClass.SetNamespace("default")

	// Check if the EC2NodeClass already exists
	err := workloadClusterClient.Get(ctx, client.ObjectKey{Name: karpenterMachinePool.Name, Namespace: "default"}, ec2NodeClass)
	if err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf("failed to get existing EC2NodeClass: %w", err)
	}

	// Generate user data for Ignition
	userData := r.generateUserData(awsCluster.Spec.Region, cluster.Name, karpenterMachinePool.Name)

	// Build the EC2NodeClass spec
	spec := map[string]interface{}{
		"amiFamily": "AL2",
		"role":      karpenterMachinePool.Spec.IamInstanceProfile,
		"userData":  userData,
	}

	// Add AMI ID if specified
	if karpenterMachinePool.Spec.EC2NodeClass != nil && karpenterMachinePool.Spec.EC2NodeClass.AMIID != nil {
		spec["amiSelectorTerms"] = []map[string]interface{}{
			{
				"id": *karpenterMachinePool.Spec.EC2NodeClass.AMIID,
			},
		}
	}

	// Add security groups if specified
	if karpenterMachinePool.Spec.EC2NodeClass != nil && len(karpenterMachinePool.Spec.EC2NodeClass.SecurityGroups) > 0 {
		spec["securityGroupSelectorTerms"] = []map[string]interface{}{
			{
				"tags": map[string]string{
					"Name": karpenterMachinePool.Spec.EC2NodeClass.SecurityGroups[0], // Using first security group for now
				},
			},
		}
	}

	// Add subnets if specified
	if karpenterMachinePool.Spec.EC2NodeClass != nil && len(karpenterMachinePool.Spec.EC2NodeClass.Subnets) > 0 {
		subnetSelectorTerms := []map[string]interface{}{}
		for _, subnet := range karpenterMachinePool.Spec.EC2NodeClass.Subnets {
			subnetSelectorTerms = append(subnetSelectorTerms, map[string]interface{}{
				"id": subnet,
			})
		}
		spec["subnetSelectorTerms"] = subnetSelectorTerms
	}

	// Add tags if specified
	if karpenterMachinePool.Spec.EC2NodeClass != nil && len(karpenterMachinePool.Spec.EC2NodeClass.Tags) > 0 {
		spec["tags"] = karpenterMachinePool.Spec.EC2NodeClass.Tags
	}

	ec2NodeClass.Object["spec"] = spec

	// Create or update the EC2NodeClass
	if k8serrors.IsNotFound(err) {
		logger.Info("Creating EC2NodeClass", "name", karpenterMachinePool.Name)
		if err := workloadClusterClient.Create(ctx, ec2NodeClass); err != nil {
			return fmt.Errorf("failed to create EC2NodeClass: %w", err)
		}
	} else {
		logger.Info("Updating EC2NodeClass", "name", karpenterMachinePool.Name)
		if err := workloadClusterClient.Update(ctx, ec2NodeClass); err != nil {
			return fmt.Errorf("failed to update EC2NodeClass: %w", err)
		}
	}

	return nil
}

// createOrUpdateNodePool creates or updates the NodePool resource in the workload cluster
func (r *KarpenterMachinePoolReconciler) createOrUpdateNodePool(ctx context.Context, logger logr.Logger, workloadClusterClient client.Client, karpenterMachinePool *v1alpha1.KarpenterMachinePool) error {
	nodePoolGVR := schema.GroupVersionResource{
		Group:    "karpenter.sh",
		Version:  "v1beta1",
		Resource: "nodepools",
	}

	nodePool := &unstructured.Unstructured{}
	nodePool.SetGroupVersionKind(nodePoolGVR.GroupVersion().WithKind("NodePool"))
	nodePool.SetName(karpenterMachinePool.Name)
	nodePool.SetNamespace("default")

	// Check if the NodePool already exists
	err := workloadClusterClient.Get(ctx, client.ObjectKey{Name: karpenterMachinePool.Name, Namespace: "default"}, nodePool)
	if err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf("failed to get existing NodePool: %w", err)
	}

	// Build the NodePool spec
	spec := map[string]interface{}{
		"disruption": map[string]interface{}{
			"consolidateAfter": "30s",
		},
		"template": map[string]interface{}{
			"spec": map[string]interface{}{
				"nodeClassRef": map[string]interface{}{
					"apiVersion": "karpenter.k8s.aws/v1beta1",
					"kind":       "EC2NodeClass",
					"name":       karpenterMachinePool.Name,
				},
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
			spec["requirements"] = requirements
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

	// Create or update the NodePool
	if k8serrors.IsNotFound(err) {
		logger.Info("Creating NodePool", "name", karpenterMachinePool.Name)
		if err := workloadClusterClient.Create(ctx, nodePool); err != nil {
			return fmt.Errorf("failed to create NodePool: %w", err)
		}
	} else {
		logger.Info("Updating NodePool", "name", karpenterMachinePool.Name)
		if err := workloadClusterClient.Update(ctx, nodePool); err != nil {
			return fmt.Errorf("failed to update NodePool: %w", err)
		}
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
		Version:  "v1beta1",
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
		Version:  "v1beta1",
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
func (r *KarpenterMachinePoolReconciler) generateUserData(region, clusterName, karpenterMachinePoolName string) string {
	userData := map[string]interface{}{
		"ignition": map[string]interface{}{
			"config": map[string]interface{}{
				"merge": []map[string]interface{}{
					{
						"source":       fmt.Sprintf("s3://%s-capa-%s/%s/%s", region, clusterName, S3ObjectPrefix, karpenterMachinePoolName),
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
