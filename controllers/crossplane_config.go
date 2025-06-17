/*
Copyright 2023.

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
	"net/url"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metaerr "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	capa "sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	eks "sigs.k8s.io/cluster-api-provider-aws/v2/controlplane/eks/api/v1beta2"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"
)

const Finalizer = "crossplane-config-operator.finalizers.giantswarm.io/config-map-controller"

type ConfigMapReconciler struct {
	Client                client.Client
	BaseDomain            string
	ManagementClusterName string
}

type ClusterInfo struct {
	Name         string
	Namespace    string
	Region       string
	AWSPartition string
	VpcID        string
	RoleArn      arn.ARN

	// Only contains the primary OIDC domain. See also the plural variant below.
	OIDCDomain string

	// All service account issuer domains
	OIDCDomains []string

	SecurityGroups *crossplaneConfigValuesAWSClusterSecurityGroups
}

// SetupWithManager sets up the controller with the Manager.
func (r *ConfigMapReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&capi.Cluster{}).
		Complete(r)
}

func (r *ConfigMapReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	clusterInfo := &ClusterInfo{}

	cluster := &capi.Cluster{}
	err := r.Client.Get(ctx, req.NamespacedName, cluster)

	if err != nil {
		logger.Error(err, "failed to get cluster")
		return ctrl.Result{}, errors.WithStack(client.IgnoreNotFound(err))
	}

	if !cluster.DeletionTimestamp.IsZero() {
		logger.Info("Reconciling delete")
		return r.reconcileDelete(ctx, cluster)
	}

	if IsEKS(*cluster) {
		awsManagedControlPlane := &eks.AWSManagedControlPlane{}
		err := r.Client.Get(ctx, req.NamespacedName, awsManagedControlPlane)
		if err != nil {
			logger.Error(err, "failed to get cluster")
			return ctrl.Result{}, errors.WithStack(client.IgnoreNotFound(err))
		}

		clusterInfo.Name = awsManagedControlPlane.Name
		clusterInfo.Namespace = awsManagedControlPlane.Namespace
		clusterInfo.Region = awsManagedControlPlane.Spec.Region
		clusterInfo.AWSPartition = getPartition(clusterInfo.Region)
		clusterInfo.VpcID = awsManagedControlPlane.Spec.NetworkSpec.VPC.ID
		clusterInfo.RoleArn, err = r.getRoleArn(ctx, awsManagedControlPlane.Spec.IdentityRef.Name, awsManagedControlPlane.Namespace)
		if err != nil {
			logger.Error(err, "failed to get cluster role identity")
			return ctrl.Result{}, errors.WithStack(client.IgnoreNotFound(err))
		}
		eksId, err := getEKSId(awsManagedControlPlane.Spec.ControlPlaneEndpoint.Host)
		if err != nil {
			logger.Error(err, "failed to get EKS Cluster ID")
			return ctrl.Result{}, errors.WithStack(client.IgnoreNotFound(err))
		}

		dnsSuffix := "amazonaws.com"

		if clusterInfo.Region == "cn-north-1" || clusterInfo.Region == "cn-northwest-1" {
			dnsSuffix = "amazonaws.com.cn"
		}

		clusterInfo.OIDCDomain = "oidc.eks." + clusterInfo.Region + "." + dnsSuffix + "/id/" + eksId
		clusterInfo.OIDCDomains = []string{clusterInfo.OIDCDomain}

	} else {
		awsCluster := &capa.AWSCluster{}
		err = r.Client.Get(ctx, req.NamespacedName, awsCluster)
		if err != nil {
			logger.Error(err, "failed to get cluster")
			return ctrl.Result{}, errors.WithStack(client.IgnoreNotFound(err))
		}
		clusterInfo.Name = awsCluster.Name
		clusterInfo.Namespace = awsCluster.Namespace
		clusterInfo.Region = awsCluster.Spec.Region
		clusterInfo.AWSPartition = getPartition(clusterInfo.Region)
		clusterInfo.VpcID = awsCluster.Spec.NetworkSpec.VPC.ID
		clusterInfo.RoleArn, err = r.getRoleArn(ctx, awsCluster.Spec.IdentityRef.Name, awsCluster.Namespace)
		if err != nil {
			logger.Error(err, "failed to get cluster role identity")
			return ctrl.Result{}, errors.WithStack(client.IgnoreNotFound(err))
		}

		// May not apply to all clusters (e.g. different in China region), so we prefer reading the actual values
		// from the `AWSCluster` annotation
		computedIRSADomain := "irsa." + clusterInfo.Name + "." + r.BaseDomain
		irsaTrustDomains := getIRSATrustDomains(awsCluster, computedIRSADomain)
		clusterInfo.OIDCDomain = irsaTrustDomains[0]
		clusterInfo.OIDCDomains = irsaTrustDomains

		clusterInfo.SecurityGroups = &crossplaneConfigValuesAWSClusterSecurityGroups{}

		if sg, ok := awsCluster.Status.Network.SecurityGroups[capa.SecurityGroupControlPlane]; ok {
			clusterInfo.SecurityGroups.ControlPlane = &crossplaneConfigValuesAWSClusterSecurityGroup{
				ID: sg.ID,
			}
		}

		if sg, ok := awsCluster.Status.Network.SecurityGroups[capa.SecurityGroupNode]; ok {
			clusterInfo.SecurityGroups.Node = &crossplaneConfigValuesAWSClusterSecurityGroup{
				ID: sg.ID,
			}
		}
	}

	return r.reconcileNormal(ctx, clusterInfo)
}

func IsEKS(cluster capi.Cluster) bool {
	return cluster.Spec.ControlPlaneRef != nil &&
		cluster.Spec.ControlPlaneRef.Kind == "AWSManagedControlPlane"
}

func getEKSId(urlString string) (string, error) {
	u, err := url.Parse(urlString)
	if err != nil {
		return "", err
	}

	// The host part of the URL is in the form of "ED3AA07D016EA49EEBC31AB274E7F3DD.sk1.eu-west-2.eks.amazonaws.com"
	// We can split it by '.' and take the first part
	parts := strings.Split(u.Hostname(), ".")
	if len(parts) > 0 {
		return parts[0], nil
	}

	return "", fmt.Errorf("unable to extract ID from URL")
}

func getIRSATrustDomains(awsCluster *capa.AWSCluster, fallbackComputedDomain string) []string {
	annotations := awsCluster.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	if s := annotations["aws.giantswarm.io/irsa-trust-domains"]; s != "" {
		irsaTrustDomains := []string{}
		values := strings.Split(s, ",")
		for _, value := range values {
			value = strings.TrimSpace(value)
			if value != "" && !slices.Contains(irsaTrustDomains, value) {
				irsaTrustDomains = append(irsaTrustDomains, value)
			}
		}
		if len(irsaTrustDomains) > 0 {
			return irsaTrustDomains
		}
	}
	return []string{fallbackComputedDomain}
}

func (r *ConfigMapReconciler) getRoleArn(ctx context.Context, idRef string, namespace string) (arn.ARN, error) {
	logger := log.FromContext(ctx)
	identity := &capa.AWSClusterRoleIdentity{}
	err := r.Client.Get(
		ctx,
		types.NamespacedName{
			Name:      idRef,
			Namespace: namespace,
		},
		identity,
	)
	if err != nil {
		return arn.ARN{}, errors.WithStack(err)
	}

	roleARN, err := arn.Parse(identity.Spec.RoleArn)
	if err != nil {
		logger.Error(err, "failed to parse role arn")
		return arn.ARN{}, errors.WithStack(err)
	}

	return roleARN, nil
}

func (r *ConfigMapReconciler) reconcileNormal(ctx context.Context, clusterInfo *ClusterInfo) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling")
	defer logger.Info("Done reconciling")

	capiCluster := &capi.Cluster{}
	err := r.Client.Get(ctx, types.NamespacedName{
		Name:      clusterInfo.Name,
		Namespace: clusterInfo.Namespace,
	}, capiCluster)
	if err != nil {
		logger.Error(err, "failed to get cluster")
		return ctrl.Result{}, errors.WithStack(client.IgnoreNotFound(err))
	}

	err = r.AddFinalizer(ctx, capiCluster)
	if err != nil {
		logger.Error(err, "failed to add finalizer")
		return ctrl.Result{}, errors.WithStack(err)
	}

	err = r.reconcileConfigMap(ctx, clusterInfo, clusterInfo.RoleArn.AccountID, r.BaseDomain)
	if err != nil {
		logger.Error(err, "failed to reconcile config map")
		return ctrl.Result{}, errors.WithStack(err)

	}

	err = r.reconcileProviderConfig(ctx, clusterInfo, clusterInfo.RoleArn.AccountID, getProviderRole(r.ManagementClusterName))
	if err != nil {
		logger.Error(err, "failed to reconcile provider config")
		return ctrl.Result{}, errors.WithStack(err)

	}

	return ctrl.Result{}, nil
}

func getProviderRole(managementClusterName string) string {
	return fmt.Sprintf("giantswarm-%s-capa-controller", managementClusterName)
}

type crossplaneConfigValues struct {
	AccountID    string                           `json:"accountID"`
	AWSCluster   crossplaneConfigValuesAWSCluster `json:"awsCluster"`
	AWSPartition string                           `json:"awsPartition"`
	BaseDomain   string                           `json:"baseDomain"`
	ClusterName  string                           `json:"clusterName"`
	Region       string                           `json:"region"`

	// For backward compatibility, we still export the primary domain as singular-named field `oidcDomain`
	OIDCDomain  string   `json:"oidcDomain"`
	OIDCDomains []string `json:"oidcDomains"`
}

type crossplaneConfigValuesAWSCluster struct {
	// Filled once available
	VpcID          string                                          `json:"vpcId,omitempty"`
	SecurityGroups *crossplaneConfigValuesAWSClusterSecurityGroups `json:"securityGroups,omitempty"`
}

type crossplaneConfigValuesAWSClusterSecurityGroups struct {
	// Filled once available
	ControlPlane *crossplaneConfigValuesAWSClusterSecurityGroup `json:"controlPlane,omitempty"`
	Node         *crossplaneConfigValuesAWSClusterSecurityGroup `json:"node,omitempty"`
}

type crossplaneConfigValuesAWSClusterSecurityGroup struct {
	ID string `json:"id"`
}

func (r *ConfigMapReconciler) reconcileConfigMap(ctx context.Context, clusterInfo *ClusterInfo, accountID, baseDomain string) error {
	config := &corev1.ConfigMap{}
	err := r.Client.Get(ctx,
		types.NamespacedName{
			Name:      fmt.Sprintf("%s-crossplane-config", clusterInfo.Name),
			Namespace: clusterInfo.Namespace,
		},
		config,
	)

	if k8serrors.IsNotFound(err) {
		return r.createConfigMap(ctx, clusterInfo, accountID, baseDomain)
	}

	return r.updateConfigMap(ctx, clusterInfo, config, accountID, baseDomain)
}

func (r *ConfigMapReconciler) reconcileProviderConfig(ctx context.Context, clusterInfo *ClusterInfo, accountID, providerRole string) error {
	logger := log.FromContext(ctx)

	providerConfig := getProviderConfig(clusterInfo.Name, clusterInfo.Namespace)

	err := r.Client.Get(ctx, types.NamespacedName{
		Name:      clusterInfo.Name,
		Namespace: clusterInfo.Namespace,
	}, providerConfig)
	if metaerr.IsNoMatchError(err) {
		logger.Info("Provider config CRD not found, skipping provider config creation")
		return nil
	}
	if k8serrors.IsNotFound(err) {
		return r.createProviderConfig(ctx, providerConfig, accountID, clusterInfo.Region, providerRole)
	}
	if err != nil {
		logger.Error(err, "Failed to get provider config")
		return errors.WithStack(err)
	}

	return r.updateProviderConfig(ctx, providerConfig, accountID, clusterInfo.Region, providerRole)
}

func (r *ConfigMapReconciler) reconcileDelete(ctx context.Context, cluster *capi.Cluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconcile delete")
	defer logger.Info("Done deleting")

	config := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-crossplane-config", cluster.Name),
			Namespace: cluster.Namespace,
		},
	}

	logger.Info("Deleting ConfigMap")
	err := r.Client.Delete(ctx, config)
	if err != nil && !k8serrors.IsNotFound(err) {
		logger.Error(err, "failed to delete config map")
		return ctrl.Result{}, errors.WithStack(err)
	}

	providerConfig := getProviderConfig(cluster.Name, cluster.Namespace)
	logger.Info("Deleting ProviderConfig")
	err = r.Client.Delete(ctx, providerConfig)
	if err != nil &&
		!k8serrors.IsNotFound(err) &&
		!metaerr.IsNoMatchError(err) {

		logger.Error(err, "failed to delete provider config")
		return ctrl.Result{}, errors.WithStack(err)
	}

	logger.Info("Removing Finalizer")
	err = r.RemoveFinalizer(ctx, cluster)
	if err != nil {
		logger.Error(err, "failed to remove finalizer")
		return ctrl.Result{}, errors.WithStack(err)
	}

	return ctrl.Result{}, nil
}

func (r *ConfigMapReconciler) AddFinalizer(ctx context.Context, cluster *capi.Cluster) error {
	originalCluster := cluster.DeepCopy()
	controllerutil.AddFinalizer(cluster, Finalizer)
	return r.Client.Patch(ctx, cluster, client.MergeFrom(originalCluster))
}

func (r *ConfigMapReconciler) RemoveFinalizer(ctx context.Context, cluster *capi.Cluster) error {

	// Check if there is an AWSCluster with the same name and namespace, and remove the finalizer. This enables the migration of the finalizer from `AWSCluster` to `Cluster`.
	awsCluster := &capa.AWSCluster{}
	err := r.Client.Get(ctx, types.NamespacedName{
		Name:      cluster.Name,
		Namespace: cluster.Namespace,
	}, awsCluster)
	if client.IgnoreNotFound(err) != nil {
		return err
	}
	if err == nil {
		originalAWSCluster := awsCluster.DeepCopy()
		controllerutil.RemoveFinalizer(awsCluster, Finalizer)
		err = r.Client.Patch(ctx, awsCluster, client.MergeFrom(originalAWSCluster))
		if err != nil {
			return err
		}
	}

	originalCluster := cluster.DeepCopy()
	controllerutil.RemoveFinalizer(cluster, Finalizer)
	err = r.Client.Patch(ctx, cluster, client.MergeFrom(originalCluster))
	if err != nil {
		return err
	}

	return err
}

func (r *ConfigMapReconciler) createConfigMap(ctx context.Context, clusterInfo *ClusterInfo, accountID, baseDomain string) error {
	logger := log.FromContext(ctx)

	logger.Info("Creating config map")
	configMapValues, err := getConfigMapValues(clusterInfo, accountID, baseDomain)
	if err != nil {
		return errors.WithStack(err)
	}

	config := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-crossplane-config", clusterInfo.Name),
			Namespace: clusterInfo.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "aws-crossplane-cluster-config-operator",
			},
		},
		Data: map[string]string{
			"values": configMapValues,
		},
	}

	err = r.Client.Create(ctx, config)
	if k8serrors.IsAlreadyExists(err) {
		logger.Info("config map already exists")
		return errors.WithStack(err)
	}
	if err != nil {
		logger.Error(err, "failed to create config map")
		return errors.WithStack(err)
	}

	return nil
}

func (r *ConfigMapReconciler) updateConfigMap(ctx context.Context,
	clusterInfo *ClusterInfo,
	config *corev1.ConfigMap,
	accountID, baseDomain string,
) error {
	logger := log.FromContext(ctx)

	configMapValues, err := getConfigMapValues(clusterInfo, accountID, baseDomain)
	if err != nil {
		return errors.WithStack(err)
	}
	patchedConfig := config.DeepCopy()
	patchedConfig.Data["values"] = configMapValues

	err = r.Client.Patch(ctx, patchedConfig, client.MergeFrom(config))
	if err != nil {
		logger.Error(err, "failed to patch config map")
		return errors.WithStack(err)
	}

	return nil
}

func (r *ConfigMapReconciler) createProviderConfig(ctx context.Context, providerConfig *unstructured.Unstructured, accountID, region, providerRole string) error {
	logger := log.FromContext(ctx)

	providerConfig.Object["spec"] = r.getProviderConfigSpec(accountID, region, providerRole)

	err := r.Client.Create(ctx, providerConfig)
	if k8serrors.IsAlreadyExists(err) {
		logger.Info("provider config already exists")
		return nil
	}
	if err != nil {
		logger.Error(err, "failed to create provider config")
		return errors.WithStack(err)
	}

	return nil
}

func (r *ConfigMapReconciler) updateProviderConfig(ctx context.Context, providerConfig *unstructured.Unstructured, accountID, region, providerRole string) error {
	logger := log.FromContext(ctx)

	patchedConfig := providerConfig.DeepCopy()
	patchedConfig.Object["spec"] = r.getProviderConfigSpec(accountID, region, providerRole)
	err := r.Client.Patch(ctx, patchedConfig, client.MergeFrom(providerConfig))
	if err != nil {
		logger.Error(err, "Failed to patch provider config")
		return errors.WithStack(err)
	}

	return nil
}

func (r *ConfigMapReconciler) getProviderConfigSpec(accountID, region, providerRole string) map[string]interface{} {
	partition := getPartition(region)
	return map[string]interface{}{
		"credentials": map[string]interface{}{
			"source": "WebIdentity",
			"webIdentity": map[string]interface{}{
				"roleARN": fmt.Sprintf("arn:%s:iam::%s:role/%s", partition, accountID, providerRole),
			},
		},
	}
}

func getConfigMapValues(clusterInfo *ClusterInfo, accountID, baseDomain string) (string, error) {
	valuesAWSCluster := crossplaneConfigValuesAWSCluster{}
	valuesAWSCluster.VpcID = clusterInfo.VpcID
	valuesAWSCluster.SecurityGroups = clusterInfo.SecurityGroups

	values := crossplaneConfigValues{
		AccountID:    accountID,
		AWSCluster:   valuesAWSCluster,
		AWSPartition: clusterInfo.AWSPartition,
		BaseDomain:   fmt.Sprintf("%s.%s", clusterInfo.Name, baseDomain),
		ClusterName:  clusterInfo.Name,
		Region:       clusterInfo.Region,
		OIDCDomain:   clusterInfo.OIDCDomains[0],
		OIDCDomains:  clusterInfo.OIDCDomains,
	}

	configMapValues, err := yaml.Marshal(values)
	if err != nil {
		return "", errors.WithStack(err)
	}

	return string(configMapValues), nil
}

func getProviderConfig(name string, namespace string) *unstructured.Unstructured {
	providerConfig := &unstructured.Unstructured{}
	providerConfig.Object = map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
		},
	}
	providerConfig.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "aws.upbound.io",
		Kind:    "ProviderConfig",
		Version: "v1beta1",
	})

	return providerConfig
}

func getPartition(region string) string {
	if strings.HasPrefix(region, "cn-") {
		return "aws-cn"
	}
	return "aws"
}
