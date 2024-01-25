package k8sclient

import (
	"context"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	capa "sigs.k8s.io/cluster-api-provider-aws/api/v1beta1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type AWSClusterClient struct {
	client client.Client
}

func NewAWSClusterClient(client client.Client) *AWSClusterClient {
	return &AWSClusterClient{
		client: client,
	}
}

func (a *AWSClusterClient) GetAWSCluster(ctx context.Context, namespacedName types.NamespacedName) (*capa.AWSCluster, error) {
	awsCluster := &capa.AWSCluster{}
	err := a.client.Get(ctx, namespacedName, awsCluster)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return awsCluster, errors.WithStack(err)
}

func (a *AWSClusterClient) GetCluster(ctx context.Context, namespacedName types.NamespacedName) (*capi.Cluster, error) {
	cluster := &capi.Cluster{}
	err := a.client.Get(ctx, namespacedName, cluster)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return cluster, errors.WithStack(err)
}

func (a *AWSClusterClient) GetOwner(ctx context.Context, awsCluster *capa.AWSCluster) (*capi.Cluster, error) {
	cluster, err := util.GetOwnerCluster(ctx, a.client, awsCluster.ObjectMeta)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return cluster, nil
}

func (a *AWSClusterClient) AddFinalizer(ctx context.Context, awsCluster *capa.AWSCluster, finalizer string) error {
	originalCluster := awsCluster.DeepCopy()
	controllerutil.AddFinalizer(awsCluster, finalizer)
	return a.client.Patch(ctx, awsCluster, client.MergeFrom(originalCluster))
}

func (a *AWSClusterClient) RemoveFinalizer(ctx context.Context, awsCluster *capa.AWSCluster, finalizer string) error {
	originalCluster := awsCluster.DeepCopy()
	controllerutil.RemoveFinalizer(awsCluster, finalizer)
	return a.client.Patch(ctx, awsCluster, client.MergeFrom(originalCluster))
}

func (a *AWSClusterClient) GetIdentity(ctx context.Context, awsCluster *capa.AWSCluster) (*capa.AWSClusterRoleIdentity, error) {
	if awsCluster.Spec.IdentityRef == nil {
		return nil, nil
	}

	roleIdentity := &capa.AWSClusterRoleIdentity{}
	err := a.client.Get(ctx, client.ObjectKey{Namespace: awsCluster.Namespace, Name: awsCluster.Spec.IdentityRef.Name}, roleIdentity)
	if err != nil {
		return &capa.AWSClusterRoleIdentity{}, errors.WithStack(err)
	}

	return roleIdentity, nil
}

func (a *AWSClusterClient) MarkConditionTrue(ctx context.Context, awsCluster *capa.AWSCluster, condition capi.ConditionType) error {
	originalCluster := awsCluster.DeepCopy()
	conditions.MarkTrue(awsCluster, condition)
	return a.client.Status().Patch(ctx, awsCluster, client.MergeFrom(originalCluster))
}

func (a *AWSClusterClient) Unpause(ctx context.Context, awsCluster *capa.AWSCluster, cluster *capi.Cluster) error {
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

func (a *AWSClusterClient) PatchCluster(ctx context.Context, cluster *capa.AWSCluster, patch client.Patch) (*capa.AWSCluster, error) {
	err := a.client.Patch(ctx, cluster, patch, &client.PatchOptions{})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return cluster, errors.WithStack(err)
}

func (a *AWSClusterClient) UpdateStatus(ctx context.Context, obj client.Object) error {
	return a.client.Status().Update(ctx, obj)
}
