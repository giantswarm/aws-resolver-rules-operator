package resolver

import (
	"github.com/pkg/errors"
)

type DNSServer struct {
	// AWSAccountId is the AWS account id where the DNS server is deployed.
	AWSAccountId string
	// IAMExternalId is the AWS external id used to be able to assume the IAM Role passed in `IAMRoleToAssume`.
	IAMExternalId string
	// AWSRegion is the AWS region where the DNS server is deployed.
	AWSRegion string
	// IAMRoleToAssume is the ARN for the IAM Role assumed to associate the Resolver Rule to the DNS server AWS account.
	IAMRoleToAssume string
	// VPCId is the AWS VPC id where the DNS server is deployed.
	VPCId string
}

func NewDNSServer(awsAccountId, iamExternalId, awsRegion, iamRoleToAssume, vpcId string) (DNSServer, error) {
	if awsAccountId == "" {
		return DNSServer{}, errors.New("AWS Account id can't be empty")
	}
	if iamExternalId == "" {
		return DNSServer{}, errors.New("IAM External id can't be empty")
	}
	if awsRegion == "" {
		return DNSServer{}, errors.New("AWS Region can't be empty")
	}
	if iamRoleToAssume == "" {
		return DNSServer{}, errors.New("AWS IAM role can't be empty")
	}
	if vpcId == "" {
		return DNSServer{}, errors.New("AWS VPC id can't be empty")
	}

	return DNSServer{
		AWSAccountId:    awsAccountId,
		IAMExternalId:   iamExternalId,
		AWSRegion:       awsRegion,
		IAMRoleToAssume: iamRoleToAssume,
		VPCId:           vpcId,
	}, nil
}
