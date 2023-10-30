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
	capa "sigs.k8s.io/cluster-api-provider-aws/api/v1beta1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/aws-resolver-rules-operator/pkg/aws"
	"github.com/aws-resolver-rules-operator/pkg/util/annotations"
	gsannotation "github.com/giantswarm/k8smetadata/pkg/annotation"
)

// RouteReconciler reconciles a Route object
type RouteReconciler struct {
	clusterClient ClusterClient
	routeClient   RouteClient
}

type RouteClient interface {
	AddRoutes(ctx context.Context, transitGatewayID, prefixListID *string, awsCluster *capa.AWSCluster, roleArn string, logger logr.Logger) error
	RemoveRoutes(ctx context.Context, transitGatewayID, prefixListID *string, awsCluster *capa.AWSCluster, roleArn string, logger logr.Logger) error
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

	transitGatewayID, err := aws.GetARNResourceID(annotations.GetNetworkTopologyTransitGateway(cluster))
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	} else if transitGatewayID == "" {
		logger.Info("transitGatewayID is not set yet, skipping")
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
	}

	prefixListID, err := aws.GetARNResourceID(annotations.GetNetworkTopologyPrefixList(cluster))
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	} else if prefixListID == "" {
		logger.Info("prefixListID is not set yet, skipping")
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
	}

	roleARN := identity.Spec.RoleArn

	if !cluster.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, &transitGatewayID, &prefixListID, awsCluster, roleARN)
	}

	return r.reconcileNormal(ctx, &transitGatewayID, &prefixListID, awsCluster, roleARN)
}

func (r *RouteReconciler) reconcileNormal(ctx context.Context, transitGatewayID, prefixListID *string, awsCluster *capa.AWSCluster, roleArn string) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Adding routes")
	if err := r.routeClient.AddRoutes(ctx, transitGatewayID, prefixListID, awsCluster, roleArn, logger); err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}
	return ctrl.Result{}, nil
}

func (r *RouteReconciler) reconcileDelete(ctx context.Context, transitGatewayID, prefixListID *string, awsCluster *capa.AWSCluster, roleArn string) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Deletig routes")
	if err := r.routeClient.RemoveRoutes(ctx, transitGatewayID, prefixListID, awsCluster, roleArn, logger); err != nil {
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
