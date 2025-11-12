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

package main

import (
	"context"
	"flag"
	"os"
	"time"

	"go.uber.org/zap/zapcore"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/cluster-api/controllers/remote"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	capa "sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	eks "sigs.k8s.io/cluster-api-provider-aws/v2/controlplane/eks/api/v1beta2"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	capiexp "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/aws-resolver-rules-operator/api/v1alpha1"
	"github.com/aws-resolver-rules-operator/controllers"
	"github.com/aws-resolver-rules-operator/pkg/aws"
	"github.com/aws-resolver-rules-operator/pkg/k8sclient"
	"github.com/aws-resolver-rules-operator/pkg/resolver"
	// +kubebuilder:scaffold:imports
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(capi.AddToScheme(scheme))
	utilruntime.Must(capiexp.AddToScheme(scheme))
	utilruntime.Must(capa.AddToScheme(scheme))
	utilruntime.Must(eks.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

type reconcilerConfig struct {
	dnsServerAWSAccountId      string
	dnsServerIAMRoleArn        string
	dnsServerIAMRoleExternalId string
	dnsServerRegion            string
	dnsServerVpcId             string
	managementClusterName      string
	managementClusterNamespace string
	workloadClusterBaseDomain  string

	awsClusterClient controllers.AWSClusterClient
	clusterClient    controllers.ClusterClient
	awsClients       resolver.AWSClients
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var disableResolverControllers bool
	var dnsServerAWSAccountId string
	var dnsServerIAMRoleArn string
	var dnsServerIAMRoleExternalId string
	var dnsServerRegion string
	var dnsServerVpcId string
	var managementClusterName string
	var managementClusterNamespace string
	var workloadClusterBaseDomain string
	var syncPeriod time.Duration
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.StringVar(&dnsServerAWSAccountId, "dns-server-aws-account-id", "", "The AWS account id where the DNS server is.")
	flag.StringVar(&dnsServerIAMRoleArn, "dns-server-iam-role-arn", "", "Assumed AWS IAM Role to associate the resolver rules.")
	flag.StringVar(&dnsServerIAMRoleExternalId, "dns-server-iam-role-external-id", "", "The IAM external id used when assuming the role passed in 'dns-server-iam-role-arn'.")
	flag.StringVar(&dnsServerRegion, "dns-server-region", "", "The AWS Region where the DNS server is.")
	flag.StringVar(&dnsServerVpcId, "dns-server-vpc-id", "", "The AWS VPC where the DNS server is.")
	flag.BoolVar(&disableResolverControllers, "disable-resolver-controllers", false,
		"Disable the resolver controllers.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&managementClusterName, "management-cluster-name", "", "Management cluster CR name.")
	flag.StringVar(&managementClusterNamespace, "management-cluster-namespace", "", "Management cluster CR namespace.")
	flag.StringVar(&workloadClusterBaseDomain, "basedomain", "", "Domain for workload cluster, e.g. installation.eu-west-1.aws.domain.tld")
	flag.DurationVar(&syncPeriod, "sync-period", 2*time.Minute, "The minimum interval at which watched resources are reconciled.")

	opts := zap.Options{
		Development: false,
		TimeEncoder: zapcore.RFC3339TimeEncoder,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	logger := zap.New(zap.UseFlagOptions(&opts))
	ctrl.SetLogger(logger)

	restConfig := ctrl.GetConfigOrDie()
	restConfig.UserAgent = "giantswarm-capa-operator"
	mgr, err := ctrl.NewManager(restConfig, ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: metricsAddr,
		},
		WebhookServer: webhook.NewServer(
			webhook.Options{
				Port: 9443,
			},
		),
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "4bb498d1.cluster.x-k8s.io",
		Cache: cache.Options{
			SyncPeriod: &syncPeriod,
		},
		// We don't want to cache Secrets
		Client: client.Options{
			Cache: &client.CacheOptions{
				DisableFor: []client.Object{
					&corev1.ConfigMap{},
					&corev1.Secret{},
				},
			},
		},
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	k8sAwsClusterClient := k8sclient.NewAWSClusterClient(mgr.GetClient())
	k8sClusterClient := k8sclient.NewClusterClient(mgr.GetClient())

	awsClients := aws.NewClients(os.Getenv("AWS_ENDPOINT"))
	cfg := reconcilerConfig{
		dnsServerAWSAccountId:      dnsServerAWSAccountId,
		dnsServerIAMRoleArn:        dnsServerIAMRoleArn,
		dnsServerIAMRoleExternalId: dnsServerIAMRoleExternalId,
		dnsServerRegion:            dnsServerRegion,
		dnsServerVpcId:             dnsServerVpcId,
		managementClusterName:      managementClusterName,
		managementClusterNamespace: managementClusterNamespace,
		workloadClusterBaseDomain:  workloadClusterBaseDomain,
		awsClusterClient:           k8sAwsClusterClient,
		clusterClient:              k8sClusterClient,
		awsClients:                 awsClients,
	}

	// TODO: This is intended for use during acceptance tests as the resolver
	// rules controllers currently don't have any tests and we don't want to
	// enable them. This should be removed when we implement those tests, but
	// maybe in the future we'd need some more granular way of disabling
	// controllers as this project grows.
	if !disableResolverControllers {
		logger.Info("Wiring resolver rules reconcilers")
		wireResolverRulesReconciler(cfg, mgr)
	}

	logger.Info("Wiring network topology reconcilers")
	wireNetworkTopologyReconcilers(cfg, mgr)

	ctx := context.Background()

	if err := (controllers.NewKarpenterMachinepoolReconciler(mgr.GetClient(), remote.NewClusterClient, cfg.awsClients)).SetupWithManager(ctx, mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "userdata")
		os.Exit(1)
	}

	if err = (&controllers.CrossplaneClusterConfigReconciler{
		Client:                mgr.GetClient(),
		BaseDomain:            workloadClusterBaseDomain,
		ManagementClusterName: managementClusterName,
	}).SetupWithManager(ctx, mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Frigate")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func wireResolverRulesReconciler(cfg reconcilerConfig, mgr manager.Manager) {
	dnsserver, err := resolver.NewDNSServer(
		cfg.dnsServerAWSAccountId,
		cfg.dnsServerIAMRoleExternalId,
		cfg.dnsServerRegion,
		cfg.dnsServerIAMRoleArn,
		cfg.dnsServerVpcId,
	)
	if err != nil {
		setupLog.Error(err, "unable to create DNSServer object")
		os.Exit(1)
	}

	awsResolver, err := resolver.NewResolver(cfg.awsClients, dnsserver)
	if err != nil {
		setupLog.Error(err, "unable to create Resolver")
		os.Exit(1)
	}

	dns, err := resolver.NewDnsZone(cfg.awsClients)
	if err != nil {
		setupLog.Error(err, "unable to create Resolver")
		os.Exit(1)
	}

	if err = controllers.NewResolverRulesReconciler(cfg.awsClusterClient, awsResolver, cfg.workloadClusterBaseDomain).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller")
		os.Exit(1)
	}

	if err = (controllers.NewUnpauseReconciler(cfg.awsClusterClient)).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller")
		os.Exit(1)
	}

	if err = controllers.NewDnsReconciler(cfg.clusterClient, dns, cfg.managementClusterName, cfg.managementClusterNamespace, cfg.workloadClusterBaseDomain).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "DnsReconciler")
		os.Exit(1)
	}

	if err = controllers.NewEKSDnsReconciler(cfg.clusterClient, dns, cfg.managementClusterName, cfg.managementClusterNamespace, cfg.workloadClusterBaseDomain).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "EKSDnsReconciler")
		os.Exit(1)
	}
}

func wireNetworkTopologyReconcilers(cfg reconcilerConfig, mgr manager.Manager) {
	managementCluster := types.NamespacedName{
		Namespace: cfg.managementClusterNamespace,
		Name:      cfg.managementClusterName,
	}

	routeReconciler := controllers.NewRouteReconciler(
		managementCluster,
		cfg.clusterClient,
		cfg.awsClients,
	)
	if err := routeReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Route")
		os.Exit(1)
	}

	mcTransitGatewayReconciler := controllers.NewManagementClusterTransitGateway(
		managementCluster,
		cfg.awsClusterClient,
		cfg.awsClients,
	)
	if err := mcTransitGatewayReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ManagementClusterTransitGateway")
		os.Exit(1)
	}

	tgwAttachmentReconciler := controllers.NewTransitGatewayAttachmentReconciler(
		managementCluster,
		cfg.awsClusterClient,
		cfg.awsClients,
	)
	if err := tgwAttachmentReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "TransitGatewayAttachment")
		os.Exit(1)
	}

	prefixListEntryReconciler := controllers.NewPrefixListEntryReconciler(
		managementCluster,
		cfg.awsClusterClient,
		cfg.awsClients,
	)
	if err := prefixListEntryReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PrefixListEntry")
		os.Exit(1)
	}

	shareReconciler := controllers.NewShareReconciler(
		managementCluster,
		cfg.awsClusterClient,
		cfg.awsClients,
	)
	if err := shareReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Share")
		os.Exit(1)
	}
}
