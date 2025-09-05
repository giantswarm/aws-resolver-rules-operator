package fixture

import (
	"context"
	"fmt"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ram"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	capa "sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gsannotations "github.com/giantswarm/k8smetadata/pkg/annotation"

	"github.com/aws-resolver-rules-operator/controllers"
	"github.com/aws-resolver-rules-operator/pkg/aws"
	"github.com/aws-resolver-rules-operator/pkg/util/annotations"
)

const (
	ClusterVCPCIDR    = "172.64.0.0/16"
	ClusterSubnetCIDR = "172.64.0.0/20"
)

type Data struct {
	Config  Config  `json:"config"`
	Network Network `json:"network"`
}

func NewFixture(k8sClient client.Client, config Config) *Fixture {
	cfg, err := awsconfig.LoadDefaultConfig(context.TODO(),
		awsconfig.WithRegion(config.AWSRegion),
	)
	Expect(err).NotTo(HaveOccurred())

	stsClient := sts.NewFromConfig(cfg)

	ec2Client := ec2.NewFromConfig(cfg, func(o *ec2.Options) {
		o.Credentials = stscreds.NewAssumeRoleProvider(stsClient, config.AWSIAMRoleARN)
	})

	ramClient := ram.NewFromConfig(cfg, func(o *ram.Options) {
		o.Credentials = stscreds.NewAssumeRoleProvider(stsClient, config.AWSIAMRoleARN)
	})

	return &Fixture{
		K8sClient: k8sClient,

		EC2Client: ec2Client,
		RamClient: ramClient,
		config:    config,
	}
}

func LoadFixture(k8sClient client.Client, data Data) *Fixture {
	f := NewFixture(k8sClient, data.Config)
	f.Network = data.Network
	f.ManagementCluster = f.loadCluster(data.Network)

	return f
}

type Cluster struct {
	Cluster             *capi.Cluster
	AWSCluster          *capa.AWSCluster
	ClusterRoleIdentity *capa.AWSClusterRoleIdentity
}

type Network struct {
	AssociationID string `json:"associationID"`
	RouteTableID  string `json:"routeTableID"`
	SubnetID      string `json:"subnetID"`
	VpcID         string `json:"vpcID"`
}

type Config struct {
	AWSAccount                 string `json:"awsAccount"`
	AWSIAMRoleARN              string `json:"awsIAMRoleARN"`
	AWSRegion                  string `json:"awsRegion"`
	ManagementClusterName      string `json:"managementClusterName"`
	ManagementClusterNamespace string `json:"managementClusterNamespace"`
}

type Fixture struct {
	K8sClient client.Client
	EC2Client *ec2.Client
	RamClient *ram.Client

	ManagementCluster Cluster

	Network Network
	config  Config
}

func (f *Fixture) Setup() error {
	f.Network = f.createNetwork()
	f.ManagementCluster = f.createCluster(f.Network)

	return nil
}

func (f *Fixture) Teardown() error {
	actualCluster := &capa.AWSCluster{}
	err := f.K8sClient.Get(context.Background(), client.ObjectKeyFromObject(f.ManagementCluster.Cluster), actualCluster)
	Expect(err).NotTo(HaveOccurred())

	err = f.deleteCluster()
	if err != nil {
		defer ginkgo.Fail(fmt.Sprintf("failed to delete cluster: %v", err))
	}

	err = DeleteKubernetesObject(f.K8sClient, f.ManagementCluster.ClusterRoleIdentity)
	Expect(err).NotTo(HaveOccurred())

	transitGatewayAnnotation := annotations.GetNetworkTopologyTransitGateway(actualCluster)
	prefixListAnnotation := annotations.GetNetworkTopologyPrefixList(actualCluster)

	gatewayID := getARNID(transitGatewayAnnotation)
	prefixListID := getARNID(prefixListAnnotation)

	err = DeletePrefixList(f.EC2Client, prefixListID)
	Expect(err).To(SatisfyAny(BeNil(), MatchError(ContainSubstring("InvalidPrefixListId.NotFound"))))

	err = DetachTransitGateway(f.EC2Client, gatewayID, f.Network.VpcID)
	Expect(err).To(SatisfyAny(BeNil(), MatchError(ContainSubstring("InvalidTransitGatewayID.NotFound"))))

	Eventually(func() error {
		err := DeleteTransitGateway(f.EC2Client, gatewayID)
		return err
	}).Should(SatisfyAny(BeNil(), MatchError(ContainSubstring("InvalidTransitGatewayID.NotFound"))))

	err = DisassociateRouteTable(f.EC2Client, f.Network.AssociationID)
	Expect(err).To(SatisfyAny(BeNil(), MatchError(ContainSubstring("InvalidRouteTableAssociationID.NotFound"))))

	err = DeleteRouteTable(f.EC2Client, f.Network.RouteTableID)
	Expect(err).To(SatisfyAny(BeNil(), MatchError(ContainSubstring("InvalidRouteTableID.NotFound"))))

	err = DeleteSubnet(f.EC2Client, f.Network.SubnetID)
	Expect(err).To(SatisfyAny(BeNil(), MatchError(ContainSubstring("InvalidSubnetID.NotFound"))))

	err = DeleteVPC(f.EC2Client, f.Network.VpcID)
	Expect(err).To(SatisfyAny(BeNil(), MatchError(ContainSubstring("InvalidVpcID.NotFound"))))

	return nil
}

func (f *Fixture) createNetwork() Network {
	createVpcOutput, err := f.EC2Client.CreateVpc(context.TODO(), &ec2.CreateVpcInput{
		CidrBlock: awssdk.String(ClusterVCPCIDR),
		TagSpecifications: []ec2types.TagSpecification{
			{
				ResourceType: ec2types.ResourceTypeVpc,
				Tags: []ec2types.Tag{
					{
						Key:   awssdk.String("aws-resolver-rules-operator.giantswarm.io/tests"),
						Value: awssdk.String("acceptance"),
					},
				},
			},
		},
	})
	Expect(err).NotTo(HaveOccurred())

	vpcID := *createVpcOutput.Vpc.VpcId

	createSubnetOutput, err := f.EC2Client.CreateSubnet(context.TODO(), &ec2.CreateSubnetInput{
		CidrBlock:         awssdk.String(ClusterSubnetCIDR),
		VpcId:             awssdk.String(vpcID),
		AvailabilityZone:  awssdk.String(getAvailabilityZone(f.config.AWSRegion)),
		TagSpecifications: generateTagSpecifications(f.config.ManagementClusterName),
	})
	Expect(err).NotTo(HaveOccurred())
	subnetID := *createSubnetOutput.Subnet.SubnetId

	createRouteTableOutput, err := f.EC2Client.CreateRouteTable(context.TODO(), &ec2.CreateRouteTableInput{
		VpcId: awssdk.String(vpcID),
	})
	Expect(err).NotTo(HaveOccurred())

	routeTableID := *createRouteTableOutput.RouteTable.RouteTableId

	assocRouteTableOutput, err := f.EC2Client.AssociateRouteTable(context.TODO(), &ec2.AssociateRouteTableInput{
		RouteTableId: awssdk.String(routeTableID),
		SubnetId:     awssdk.String(subnetID),
	})
	Expect(err).NotTo(HaveOccurred())

	associationID := *assocRouteTableOutput.AssociationId

	return Network{
		VpcID:         vpcID,
		SubnetID:      subnetID,
		AssociationID: associationID,
		RouteTableID:  routeTableID,
	}
}

func (f *Fixture) createCluster(network Network) Cluster {
	ctx := context.Background()

	clusterRoleIdentity := &capa.AWSClusterRoleIdentity{
		ObjectMeta: metav1.ObjectMeta{
			Name: f.config.ManagementClusterName,
		},
		Spec: capa.AWSClusterRoleIdentitySpec{
			AWSRoleSpec: capa.AWSRoleSpec{
				RoleArn: f.config.AWSIAMRoleARN,
			},
			SourceIdentityRef: &capa.AWSIdentityReference{
				Name: "default",
				Kind: capa.ControllerIdentityKind,
			},
		},
	}

	err := f.K8sClient.Create(ctx, clusterRoleIdentity)
	Expect(err).NotTo(HaveOccurred())

	cluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      f.config.ManagementClusterName,
			Namespace: f.config.ManagementClusterNamespace,
		},
		Spec: capi.ClusterSpec{
			InfrastructureRef: &corev1.ObjectReference{
				APIVersion: capa.GroupVersion.String(),
				Kind:       "AWSCluster",
				Namespace:  f.config.ManagementClusterNamespace,
				Name:       f.config.ManagementClusterName,
			},
		},
	}

	err = f.K8sClient.Create(ctx, cluster)
	Expect(err).NotTo(HaveOccurred())

	awsCluster := &capa.AWSCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      f.config.ManagementClusterName,
			Namespace: f.config.ManagementClusterNamespace,
			Annotations: map[string]string{
				gsannotations.NetworkTopologyModeAnnotation: gsannotations.NetworkTopologyModeGiantSwarmManaged,
			},
		},
		Spec: capa.AWSClusterSpec{
			Region: f.config.AWSRegion,
			IdentityRef: &capa.AWSIdentityReference{
				Name: f.config.ManagementClusterName,
				Kind: capa.ClusterRoleIdentityKind,
			},
			NetworkSpec: capa.NetworkSpec{
				VPC: capa.VPCSpec{
					ID:        network.VpcID,
					CidrBlock: ClusterVCPCIDR,
				},
				Subnets: []capa.SubnetSpec{
					{
						ID:         "subnet-1",
						CidrBlock:  ClusterSubnetCIDR,
						ResourceID: network.SubnetID,
						IsPublic:   false,
					},
				},
			},
		},
	}

	err = f.K8sClient.Create(ctx, awsCluster)
	Expect(err).NotTo(HaveOccurred())

	return Cluster{
		Cluster:             cluster,
		AWSCluster:          awsCluster,
		ClusterRoleIdentity: clusterRoleIdentity,
	}
}

func (f *Fixture) loadCluster(network Network) Cluster {
	ctx := context.Background()

	clusterRoleIdentity := &capa.AWSClusterRoleIdentity{}
	err := f.K8sClient.Get(ctx, types.NamespacedName{
		Name: f.config.ManagementClusterName,
	}, clusterRoleIdentity)
	Expect(err).NotTo(HaveOccurred())

	cluster := &capi.Cluster{}
	err = f.K8sClient.Get(ctx, types.NamespacedName{
		Name:      f.config.ManagementClusterName,
		Namespace: f.config.ManagementClusterNamespace,
	}, cluster)
	Expect(err).NotTo(HaveOccurred())

	awsCluster := &capa.AWSCluster{}
	err = f.K8sClient.Get(ctx, types.NamespacedName{
		Name:      f.config.ManagementClusterName,
		Namespace: f.config.ManagementClusterNamespace,
	}, awsCluster)
	Expect(err).NotTo(HaveOccurred())

	return Cluster{
		Cluster:             cluster,
		AWSCluster:          awsCluster,
		ClusterRoleIdentity: clusterRoleIdentity,
	}
}

func (f *Fixture) deleteCluster() error {
	err := DeleteKubernetesObject(f.K8sClient, f.ManagementCluster.Cluster)
	Expect(err).NotTo(HaveOccurred())

	awsCluster := f.ManagementCluster.AWSCluster
	err = DeleteKubernetesObject(f.K8sClient, awsCluster)
	Expect(err).NotTo(HaveOccurred())

	if awsCluster == nil {
		return nil
	}

	timeout := time.After(3 * time.Minute)
	tick := time.NewTicker(5 * time.Second)

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for cluster deletion")
		case <-tick.C:
			actualCluster := &capi.Cluster{}
			err := f.K8sClient.Get(context.Background(), client.ObjectKeyFromObject(awsCluster), actualCluster)
			if k8serrors.IsNotFound(err) {
				return nil
			}
			if err != nil {
				return err
			}
		}
	}
}

func getAvailabilityZone(region string) string {
	return fmt.Sprintf("%sa", region)
}

func generateTagSpecifications(name string) []ec2types.TagSpecification {
	tagSpec := ec2types.TagSpecification{
		ResourceType: ec2types.ResourceTypeSubnet,
		Tags: []ec2types.Tag{
			{
				Key:   awssdk.String(controllers.TagSubnetTGWAttachements),
				Value: awssdk.String("true"),
			},
			{
				Key:   awssdk.String(capa.NameKubernetesAWSCloudProviderPrefix + name),
				Value: awssdk.String("shared"),
			},
		},
	}

	tagSpecifications := make([]ec2types.TagSpecification, 0)
	tagSpecifications = append(tagSpecifications, tagSpec)
	return tagSpecifications
}

func getARNID(arn string) string {
	if arn == "" {
		return ""
	}

	resourceID, err := aws.GetARNResourceID(arn)
	Expect(err).NotTo(HaveOccurred())
	return resourceID
}
