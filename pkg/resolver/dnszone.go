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

func BuildPrivateHostedZone(dnsName string, cluster Cluster, tags map[string]string, vpcsToAssociate []string) DnsZone {
	return DnsZone{
		DnsName:         dnsName,
		IsPrivate:       true,
		VPCId:           cluster.VPCId,
		Region:          cluster.Region,
		Tags:            tags,
		VPCsToAssociate: vpcsToAssociate,
	}
}
