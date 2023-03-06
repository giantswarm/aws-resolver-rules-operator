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

	gsannotations "github.com/giantswarm/k8smetadata/pkg/annotation"
	"github.com/pkg/errors"
	capa "sigs.k8s.io/cluster-api-provider-aws/api/v1beta1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type UnpauseReconciler struct {
	awsClusterClient AWSClusterClient
}

func NewUnpauseReconciler(awsClusterClient AWSClusterClient) *UnpauseReconciler {
	return &UnpauseReconciler{
		awsClusterClient: awsClusterClient,
	}
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=awsclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=awsclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=awsclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch

func (r *UnpauseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling")
	defer logger.Info("Done reconciling")

	awsCluster, err := r.awsClusterClient.Get(ctx, req.NamespacedName)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(client.IgnoreNotFound(err))
	}

	cluster, err := r.awsClusterClient.GetOwner(ctx, awsCluster)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	if cluster == nil {
		logger.Info("Cluster controller has not yet set OwnerRef")
		return ctrl.Result{}, nil
	}

	dnsModeAnnotation, ok := awsCluster.Annotations[gsannotations.AWSDNSMode]
	if !ok || dnsModeAnnotation != gsannotations.DNSModePrivate {
		logger.Info("AWSCluster is not using private DNS mode, skipping")
		return ctrl.Result{}, nil
	}

	if !awsCluster.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	return r.reconcileNormal(ctx, awsCluster, cluster)
}

func (r *UnpauseReconciler) reconcileNormal(ctx context.Context, awsCluster *capa.AWSCluster, cluster *capi.Cluster) (ctrl.Result, error) {
	if conditions.IsTrue(awsCluster, capa.VpcReadyCondition) && conditions.IsTrue(awsCluster, capa.SubnetsReadyCondition) && conditions.IsTrue(awsCluster, ResolverRulesAssociatedCondition) {
		err := r.awsClusterClient.Unpause(ctx, awsCluster, cluster)
		if err != nil {
			return ctrl.Result{}, errors.WithStack(err)
		}
	}

	return reconcile.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *UnpauseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("unpause").
		For(&capa.AWSCluster{}).
		WithEventFilter(predicates.ResourceNotPaused(mgr.GetLogger())).
		Complete(r)
}
