package resolver

// MachineAddressType describes a valid MachineAddress type.
type DnsRecordType string

// Define the MachineAddressType constants.
const (
	DnsRecordTypeA     DnsRecordType = "A"
	DnsRecordTypeCname DnsRecordType = "CNAME"
	DnsRecordTypeAlias DnsRecordType = "ALIAS"
	DnsRecordTypeCAA   DnsRecordType = "CAA"
)

type DNSRecord struct {
	Kind   DnsRecordType
	Name   string
	Values []string
	Region string
}
