package resolver

type Cluster struct {
	VPCsToAssociateToHostedZone []string
	Name                        string
	BastionIp                   string
	ControlPlaneEndpoint        string
	IsDnsModePrivate            bool
	IsVpcModePrivate            bool
	IsEKS                       bool
	Region                      string
	VPCCidr                     string
	VPCId                       string
	IAMRoleARN                  string
	MCIAMRoleARN                string
	Subnets                     []string
}
