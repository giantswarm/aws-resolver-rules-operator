package resolver

type FakeClients struct {
	EC2Client              EC2Client
	RAMClient              RAMClient
	ResolverClient         ResolverClient
	ExternalResolverClient ResolverClient
	Route53Client          Route53Client
	PrefixListClient       PrefixListClient
	TransitGatewayClient   TransitGatewayClient
}

func (f *FakeClients) NewResolverClient(region, roleToAssume string) (ResolverClient, error) {
	return f.ResolverClient, nil
}

func (f *FakeClients) NewEC2Client(region, arn string) (EC2Client, error) {
	return f.EC2Client, nil
}

func (f *FakeClients) NewRAMClient(region, arn string) (RAMClient, error) {
	return f.RAMClient, nil
}

func (f *FakeClients) NewResolverClientWithExternalId(region, roleToAssume, externalRoleToAssume, externalId string) (ResolverClient, error) {
	return f.ExternalResolverClient, nil
}

func (f *FakeClients) NewEC2ClientWithExternalId(region, arn, externalId string) (EC2Client, error) {
	return f.EC2Client, nil
}

func (f *FakeClients) NewRAMClientWithExternalId(region, arn, externalId string) (RAMClient, error) {
	return f.RAMClient, nil
}

func (f *FakeClients) NewRoute53Client(region, arn string) (Route53Client, error) {
	return f.Route53Client, nil
}

func (f *FakeClients) NewTransitGatewayClient(region, arn string) (TransitGatewayClient, error) {
	return f.TransitGatewayClient, nil
}

func (f *FakeClients) NewPrefixListClient(region, arn string) (PrefixListClient, error) {
	return f.PrefixListClient, nil
}
