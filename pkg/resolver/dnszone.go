package resolver

type DnsZone struct {
	DnsName         string
	IsPrivate       bool
	Region          string
	Tags            map[string]string
	VPCId           string
	VPCsToAssociate []string
}

func BuildPublicHostedZone(dnsName string, tags map[string]string) DnsZone {
	return DnsZone{
		DnsName:   dnsName,
		IsPrivate: false,
		Tags:      tags,
	}
}

func BuildPrivateHostedZone(dnsName string, tags map[string]string, vpcId, region string, vpcsToAssociate []string) DnsZone {
	return DnsZone{
		DnsName:         dnsName,
		IsPrivate:       true,
		VPCId:           vpcId,
		Region:          region,
		Tags:            tags,
		VPCsToAssociate: vpcsToAssociate,
	}
}
