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
	"flag"
	"os"

	"go.uber.org/zap/zapcore"
	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	capa "sigs.k8s.io/cluster-api-provider-aws/api/v1beta1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/aws-resolver-rules-operator/controllers"
	"github.com/aws-resolver-rules-operator/pkg/aws"
	"github.com/aws-resolver-rules-operator/pkg/k8sclient"
	"github.com/aws-resolver-rules-operator/pkg/resolver"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(capi.AddToScheme(scheme))
	utilruntime.Must(capa.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var dnsServerAWSAccountId string
	var dnsServerIAMRoleArn string
	var dnsServerIAMRoleExternalId string
	var dnsServerRegion string
	var dnsServerVpcId string
	var workloadClusterBaseDomain string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.StringVar(&dnsServerAWSAccountId, "dns-server-aws-account-id", "", "The AWS account id where the DNS server is.")
	flag.StringVar(&dnsServerIAMRoleArn, "dns-server-iam-role-arn", "", "Assumed AWS IAM Role to associate the resolver rules.")
	flag.StringVar(&dnsServerIAMRoleExternalId, "dns-server-iam-role-external-id", "", "The IAM external id used when assuming the role passed in 'dns-server-iam-role-arn'.")
	flag.StringVar(&dnsServerRegion, "dns-server-region", "", "The AWS Region where the DNS server is.")
	flag.StringVar(&dnsServerVpcId, "dns-server-vpc-id", "", "The AWS VPC where the DNS server is.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&workloadClusterBaseDomain, "basedomain", "", "Domain for workload cluster, e.g. installation.eu-west-1.aws.domain.tld")

	opts := zap.Options{
		Development: true,
		TimeEncoder: zapcore.RFC3339TimeEncoder,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "4bb498d1.cluster.x-k8s.io",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	k8sAwsClusterClient := k8sclient.NewAWSClusterClient(mgr.GetClient())
	awsClients := aws.NewClients(os.Getenv("AWS_ENDPOINT"))

	dnsserver, err := resolver.NewDNSServer(dnsServerAWSAccountId, dnsServerIAMRoleExternalId, dnsServerRegion, dnsServerIAMRoleArn, dnsServerVpcId)
	if err != nil {
		setupLog.Error(err, "unable to create DNSServer object")
		os.Exit(1)
	}

	awsResolver, err := resolver.NewResolver(awsClients, dnsserver, workloadClusterBaseDomain)
	if err != nil {
		setupLog.Error(err, "unable to create Resolver")
		os.Exit(1)
	}

	if err = (controllers.NewResolverRulesDNSServerReconciler(k8sAwsClusterClient, awsResolver)).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller Resolver Rule reconciler for DNS Server")
		os.Exit(1)
	}

	if err = (controllers.NewResolverRulesReconciler(k8sAwsClusterClient, awsResolver)).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller Resovler Rule reconciler for AWS account")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

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
