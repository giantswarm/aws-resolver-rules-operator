package k8sclient

import (
	"context"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	capa "sigs.k8s.io/cluster-api-provider-aws/api/v1beta1"
	eks "sigs.k8s.io/cluster-api-provider-aws/controlplane/eks/api/v1beta1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type ClusterClient struct {
	client client.Client
}

func NewClusterClient(client client.Client) *ClusterClient {
	return &ClusterClient{
		client: client,
	}
}

func (a *ClusterClient) GetAWSCluster(ctx context.Context, namespacedName types.NamespacedName) (*capa.AWSCluster, error) {
	awsCluster := &capa.AWSCluster{}
	err := a.client.Get(ctx, namespacedName, awsCluster)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return awsCluster, errors.WithStack(err)
}

func (a *ClusterClient) GetAWSManagedControlPlane(ctx context.Context, namespacedName types.NamespacedName) (*eks.AWSManagedControlPlane, error) {
	awsManagedControlPlane := &eks.AWSManagedControlPlane{}
	err := a.client.Get(ctx, namespacedName, awsManagedControlPlane)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return awsManagedControlPlane, errors.WithStack(err)
}

func (a *ClusterClient) GetCluster(ctx context.Context, namespacedName types.NamespacedName) (*capi.Cluster, error) {
	cluster := &capi.Cluster{}
	err := a.client.Get(ctx, namespacedName, cluster)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return cluster, errors.WithStack(err)
}

func (a *ClusterClient) AddAWSClusterFinalizer(ctx context.Context, awsCluster *capa.AWSCluster, finalizer string) error {
	originalCluster := awsCluster.DeepCopy()
	updated := controllerutil.AddFinalizer(awsCluster, finalizer)
	if updated {
		return a.client.Patch(ctx, awsCluster, client.MergeFrom(originalCluster))
	}

	return nil
}
func (a *ClusterClient) AddAWSManagedControlPlaneFinalizer(ctx context.Context, awsManagedControlPlane *eks.AWSManagedControlPlane, finalizer string) error {
	originalCluster := awsManagedControlPlane.DeepCopy()
	updated := controllerutil.AddFinalizer(awsManagedControlPlane, finalizer)
	if updated {
		return a.client.Patch(ctx, awsManagedControlPlane, client.MergeFrom(originalCluster))
	}

	return nil
}

func (a *ClusterClient) RemoveAWSClusterFinalizer(ctx context.Context, awsCluster *capa.AWSCluster, finalizer string) error {
	originalCluster := awsCluster.DeepCopy()
	controllerutil.RemoveFinalizer(awsCluster, finalizer)
	return a.client.Patch(ctx, awsCluster, client.MergeFrom(originalCluster))
}

func (a *ClusterClient) RemoveAWSManagedControlPlaneFinalizer(ctx context.Context, awsManagedControlPlane *eks.AWSManagedControlPlane, finalizer string) error {
	originalCluster := awsManagedControlPlane.DeepCopy()
	controllerutil.RemoveFinalizer(awsManagedControlPlane, finalizer)
	return a.client.Patch(ctx, awsManagedControlPlane, client.MergeFrom(originalCluster))
}

func (a *ClusterClient) GetIdentity(ctx context.Context, identityRef *capa.AWSIdentityReference) (*capa.AWSClusterRoleIdentity, error) {
	if identityRef == nil {
		return nil, nil
	}
	roleIdentity := &capa.AWSClusterRoleIdentity{}
	err := a.client.Get(ctx, client.ObjectKey{Name: identityRef.Name}, roleIdentity)
	if err != nil {
		return &capa.AWSClusterRoleIdentity{}, errors.WithStack(err)
	}

	return roleIdentity, nil
}

func (a *ClusterClient) GetBastionMachine(ctx context.Context, clusterName string) (*capi.Machine, error) {
	bastionMachineList := &capi.MachineList{}
	err := a.client.List(ctx, bastionMachineList, client.MatchingLabels{
		capi.ClusterLabelName:   clusterName,
		"cluster.x-k8s.io/role": "bastion",
	})
	if err != nil {
		return &capi.Machine{}, errors.WithStack(err)
	}

	if len(bastionMachineList.Items) < 1 {
		return &capi.Machine{}, &BastionNotFoundError{}
	}

	return &bastionMachineList.Items[0], nil
}

func (a *ClusterClient) MarkConditionTrue(ctx context.Context, cluster *capi.Cluster, condition capi.ConditionType) error {
	originalCluster := cluster.DeepCopy()
	conditions.MarkTrue(cluster, condition)
	return a.client.Status().Patch(ctx, cluster, client.MergeFrom(originalCluster))
}

func (a *ClusterClient) Unpause(ctx context.Context, awsCluster *capa.AWSCluster, cluster *capi.Cluster) error {
	originalCluster := cluster.DeepCopy()
	cluster.Spec.Paused = false
	delete(cluster.Annotations, capi.PausedAnnotation)
	err := a.client.Patch(ctx, cluster, client.MergeFrom(originalCluster))
	if err != nil {
		return errors.WithStack(err)
	}

	originalAwsCluster := awsCluster.DeepCopy()
	delete(awsCluster.Annotations, capi.PausedAnnotation)
	return a.client.Patch(ctx, awsCluster, client.MergeFrom(originalAwsCluster))
}
