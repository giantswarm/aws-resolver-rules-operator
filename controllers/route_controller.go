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

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	capa "sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/aws-resolver-rules-operator/pkg/aws"
	"github.com/aws-resolver-rules-operator/pkg/resolver"
	"github.com/aws-resolver-rules-operator/pkg/util/annotations"
	gsannotation "github.com/giantswarm/k8smetadata/pkg/annotation"
)

const (
	RouteFinalizer = "capa-operator.finalizers.giantswarm.io/route-controller"
)

// RouteReconciler reconciles a Route object
type RouteReconciler struct {
	managementCluster types.NamespacedName
	clusterClient     ClusterClient
	awsClients        resolver.AWSClients
}

func NewRouteReconciler(managementCluster types.NamespacedName, clusterClient ClusterClient, clients resolver.AWSClients) *RouteReconciler {
	return &RouteReconciler{
		managementCluster: managementCluster,
		clusterClient:     clusterClient,
		awsClients:        clients,
	}
}

// RouteReconciler reconciles AWSClusters.
// It only reconciles AWSCluster owned by Cluster which uses  UserManaged or GiantswarmManaged topology mode,
// set by the `network-topology.giantswarm.io/mode` annotation.
// It creates routes in the routing tables associated with AWSClusters's subnets
func (r *RouteReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	awsCluster, err := r.clusterClient.GetAWSCluster(ctx, req.NamespacedName)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(client.IgnoreNotFound(err))
	}

	switch val := annotations.GetAnnotation(awsCluster, gsannotation.NetworkTopologyModeAnnotation); val {
	case "":
		logger.Info("No NetworkTopologyMode annotation found on cluster, skipping")
		return ctrl.Result{}, nil
	case gsannotation.NetworkTopologyModeNone:
		logger.Info("Mode currently not handled", "NetworkTopologyMode", gsannotation.NetworkTopologyModeNone)
		return ctrl.Result{}, nil
	case gsannotation.NetworkTopologyModeUserManaged, gsannotation.NetworkTopologyModeGiantSwarmManaged:
	default:
		err := fmt.Errorf("invalid NetworkTopologyMode value")
		logger.Error(err, "Unexpected NetworkTopologyMode annotation value found on cluster", "NetworkTopologyMode", val)
		return ctrl.Result{}, errors.WithStack(err)
	}

	managementCluster, err := r.clusterClient.GetAWSCluster(ctx, r.managementCluster)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	identity, err := r.clusterClient.GetIdentity(ctx, awsCluster.Spec.IdentityRef)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	if identity == nil {
		logger.Info("AWSCluster has no identityRef set, skipping")
		return ctrl.Result{}, nil
	}

	transitGatewayARN := getTransitGatewayARN(awsCluster, managementCluster)
	if transitGatewayARN == "" {
		logger.Info("transitGatewayARN is not set yet, skipping")
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
	}

	transitGatewayID, err := aws.GetARNResourceID(transitGatewayARN)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	prefixListARN := getPrefixListARN(awsCluster, managementCluster)

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

	routeRule := resolver.RouteRule{
		DestinationPrefixListId: prefixListID,
		TransitGatewayId:        transitGatewayID,
	}

	subnetFilter := subnets

	routeTableClient, err := r.awsClients.NewRouteTableClient(awsCluster.Spec.Region, roleARN)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	if !awsCluster.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, awsCluster, routeRule, subnetFilter, routeTableClient)
	}

	return r.reconcileNormal(ctx, awsCluster, routeRule, subnetFilter, routeTableClient)
}

func (r *RouteReconciler) reconcileNormal(ctx context.Context, awsCluster *capa.AWSCluster, routeRule resolver.RouteRule, subnetFilter resolver.Filter, routeTableClient resolver.RouteTableClient) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Adding routes")

	if err := r.clusterClient.AddAWSClusterFinalizer(ctx, awsCluster, RouteFinalizer); err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	if err := routeTableClient.AddRoutes(ctx, routeRule, subnetFilter); err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}
	return ctrl.Result{}, nil
}

func (r *RouteReconciler) reconcileDelete(ctx context.Context, awsCluster *capa.AWSCluster, routeRule resolver.RouteRule, subnetFilter resolver.Filter, routeTableClient resolver.RouteTableClient) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(awsCluster, RouteFinalizer) {
		return ctrl.Result{}, nil
	}

	logger := log.FromContext(ctx)
	logger.Info("Deleting routes")

	if err := routeTableClient.RemoveRoutes(ctx, routeRule, subnetFilter); err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	if err := r.clusterClient.RemoveAWSClusterFinalizer(ctx, awsCluster, RouteFinalizer); err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RouteReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("routerules").
		For(&capa.AWSCluster{}).
		WithEventFilter(
			predicate.Funcs{
				UpdateFunc: predicateToFilterAWSClusterResourceVersionChanges,
			},
		).
		Complete(r)
}
