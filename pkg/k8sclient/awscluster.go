package k8sclient

import (
	"context"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	capa "sigs.k8s.io/cluster-api-provider-aws/api/v1beta1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type AWSCluster struct {
	client client.Client
}

func NewAWSCluster(client client.Client) *AWSCluster {
	return &AWSCluster{
		client: client,
	}
}

func (g *AWSCluster) Get(ctx context.Context, namespacedName types.NamespacedName) (*capa.AWSCluster, error) {
	awsCluster := &capa.AWSCluster{}
	err := g.client.Get(ctx, namespacedName, awsCluster)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return awsCluster, errors.WithStack(err)
}

func (g *AWSCluster) GetOwner(ctx context.Context, awsCluster *capa.AWSCluster) (*capi.Cluster, error) {
	cluster, err := util.GetOwnerCluster(ctx, g.client, awsCluster.ObjectMeta)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return cluster, nil
}

func (g *AWSCluster) AddFinalizer(ctx context.Context, awsCluster *capa.AWSCluster, finalizer string) error {
	originalCluster := awsCluster.DeepCopy()
	controllerutil.AddFinalizer(awsCluster, finalizer)
	return g.client.Patch(ctx, awsCluster, client.MergeFrom(originalCluster))
}

func (g *AWSCluster) RemoveFinalizer(ctx context.Context, awsCluster *capa.AWSCluster, finalizer string) error {
	originalCluster := awsCluster.DeepCopy()
	controllerutil.RemoveFinalizer(awsCluster, finalizer)
	return g.client.Patch(ctx, awsCluster, client.MergeFrom(originalCluster))
}
