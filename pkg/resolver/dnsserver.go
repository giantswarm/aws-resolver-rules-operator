package resolver

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
	return DNSServer{
		AWSAccountId:    awsAccountId,
		IAMExternalId:   iamExternalId,
		AWSRegion:       awsRegion,
		IAMRoleToAssume: iamRoleToAssume,
		VPCId:           vpcId,
	}, nil
}
