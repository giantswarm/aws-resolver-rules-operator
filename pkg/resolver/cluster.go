package resolver

type Cluster struct {
	AdditionalTags              map[string]string
	VPCsToAssociateToHostedZone []string
	Name                        string
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
