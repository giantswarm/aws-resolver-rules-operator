package controllers

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"path"
	"time"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
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
		logger.Info("Terminating EC2 instances for KarpenterMachinePool", "karpenterMachinePoolName", karpenterMachinePool.Name)
		err = ec2Client.TerminateInstancesByTag(ctx, logger, "karpenter.sh/nodepool", karpenterMachinePool.Name)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to terminate EC2 instances: %w", err)
		}
	}

	// Create deep copy of the reconciled object so we can change it
	karpenterMachinePoolCopy := karpenterMachinePool.DeepCopy()

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

// SetupWithManager sets up the controller with the Manager.
func (r *KarpenterMachinePoolReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	logger := capalogger.FromContext(ctx).GetLogger()

	return ctrl.NewControllerManagedBy(mgr).
		Named("karpentermachinepool").
		For(&v1alpha1.KarpenterMachinePool{}).
		WithEventFilter(predicates.ResourceNotPaused(logger)).
		Complete(r)
}
