/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/aws-resolver-rules-operator/pkg/aws"
	"github.com/aws-resolver-rules-operator/pkg/util/annotations"
	gsannotation "github.com/giantswarm/k8smetadata/pkg/annotation"
)

const (
	RouteFinalizer = "capa-operator.finalizers.giantswarm.io/route-controller"
)

// RouteReconciler reconciles a Route object
type RouteReconciler struct {
	clusterClient ClusterClient
	routeClient   RouteClient
}

//counterfeiter:generate . RouteClient
type RouteClient interface {
	AddRoutes(ctx context.Context, transitGatewayID, prefixListID *string, subnets []string, roleArn, region string, logger logr.Logger) error
	RemoveRoutes(ctx context.Context, transitGatewayID, prefixListID *string, subnets []string, roleArn, region string, logger logr.Logger) error
}

func NewRouteReconciler(clusterClient ClusterClient, route RouteClient) *RouteReconciler {
	return &RouteReconciler{
		clusterClient: clusterClient,
		routeClient:   route,
	}
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=routes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=routes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=routes/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *RouteReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	cluster, err := r.clusterClient.GetCluster(ctx, req.NamespacedName)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	switch val := annotations.GetAnnotation(cluster, gsannotation.NetworkTopologyModeAnnotation); val {
	case "":
		return ctrl.Result{}, nil
	case gsannotation.NetworkTopologyModeNone:
		logger.Info("Mode currently not handled", "mode", gsannotation.NetworkTopologyModeNone)
		return ctrl.Result{}, nil
	case gsannotation.NetworkTopologyModeUserManaged, gsannotation.NetworkTopologyModeGiantSwarmManaged:
	default:
		err := fmt.Errorf("invalid NetworkTopologyMode value")
		logger.Error(err, "Unexpected NetworkTopologyMode annotation value found on cluster", "value", val)
		return ctrl.Result{}, errors.WithStack(err)
	}

	awsCluster, err := r.clusterClient.GetAWSCluster(ctx, req.NamespacedName)
	if err != nil {
		return ctrl.Result{}, err
	}

	identity, err := r.clusterClient.GetIdentity(ctx, awsCluster.Spec.IdentityRef)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	if identity == nil {
		logger.Info("AWSCluster has no identityRef set, skipping")
		return ctrl.Result{}, nil
	}

	transitGatewayARN := annotations.GetNetworkTopologyTransitGateway(cluster)
	if transitGatewayARN == "" {
		logger.Info("transitGatewayARN is not set yet, skipping")
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
	}

	transitGatewayID, err := aws.GetARNResourceID(transitGatewayARN)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	prefixListARN := annotations.GetNetworkTopologyPrefixList(cluster)

	if prefixListARN == "" {
		logger.Info("prefixListARN is not set yet, skipping")
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
	}

	prefixListID, err := aws.GetARNResourceID(prefixListARN)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	roleARN := identity.Spec.RoleArn

	subnets := []string{}
	for _, s := range awsCluster.Spec.NetworkSpec.Subnets {
		temp := s.ID
		subnets = append(subnets, temp)
	}

	if !cluster.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, &transitGatewayID, &prefixListID, subnets, cluster, roleARN, awsCluster.Spec.Region, logger)
	}

	return r.reconcileNormal(ctx, &transitGatewayID, &prefixListID, subnets, cluster, roleARN, awsCluster.Spec.Region, logger)
}

func (r *RouteReconciler) reconcileNormal(ctx context.Context, transitGatewayID, prefixListID *string, subnets []string, cluster *capi.Cluster, roleArn, region string, logger logr.Logger) (ctrl.Result, error) {
	logger.Info("Adding routes")

	if err := r.clusterClient.AddClusterFinalizer(ctx, cluster, RouteFinalizer); err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	if err := r.routeClient.AddRoutes(ctx, transitGatewayID, prefixListID, subnets, roleArn, region, logger); err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}
	return ctrl.Result{}, nil
}

func (r *RouteReconciler) reconcileDelete(ctx context.Context, transitGatewayID, prefixListID *string, subnets []string, cluster *capi.Cluster, roleArn, region string, logger logr.Logger) (ctrl.Result, error) {
	logger.Info("Deletig routes")

	if err := r.routeClient.RemoveRoutes(ctx, transitGatewayID, prefixListID, subnets, roleArn, region, logger); err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	if err := r.clusterClient.RemoveClusterFinalizer(ctx, cluster, RouteFinalizer); err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RouteReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&capi.Cluster{}).
		Complete(r)
}
