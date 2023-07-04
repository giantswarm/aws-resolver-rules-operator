package resolver

type Cluster struct {
	VPCsToAssociateToHostedZone []string
	Name                        string
	BastionIp                   string
	ControlPlaneEndpoint        string
	IsDnsModePrivate            bool
	Region                      string
	VPCCidr                     string
	VPCId                       string
	IAMRoleARN                  string
	Subnets                     []string
}