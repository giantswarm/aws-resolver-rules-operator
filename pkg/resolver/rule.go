package resolver

import (
	"net"
)

type ResolverRule struct {
	Arn  string
	Id   string
	IPs  []string
	Name string
}

// TargetIPsBelongToCidr checks if any of rule target IPs belongs to a CIDR range.
func (r *ResolverRule) TargetIPsBelongToCidr(vpcCidr string) (bool, error) {
	_, parsedCidr, err := net.ParseCIDR(vpcCidr)
	if err != nil {
		return false, err
	}

	for _, targetIp := range r.IPs {
		parsedIP := net.ParseIP(targetIp)
		if parsedCidr.Contains(parsedIP) {
			return true, nil
		}
	}

	return false, nil
}
