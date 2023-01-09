package resolver

type FakeClients struct {
	EC2Client      EC2Client
	RAMClient      RAMClient
	ResolverClient ResolverClient
}

func (f *FakeClients) NewResolverClient(region, arn string) (ResolverClient, error) {
	return f.ResolverClient, nil
}

func (f *FakeClients) NewEC2Client(region, arn string) (EC2Client, error) {
	return f.EC2Client, nil
}

func (f *FakeClients) NewRAMClient(region, arn string) (RAMClient, error) {
	return f.RAMClient, nil
}

func (f *FakeClients) NewResolverClientWithExternalId(region, arn, externalId string) (ResolverClient, error) {
	return f.ResolverClient, nil
}

func (f *FakeClients) NewEC2ClientWithExternalId(region, arn, externalId string) (EC2Client, error) {
	return f.EC2Client, nil
}

func (f *FakeClients) NewRAMClientWithExternalId(region, arn, externalId string) (RAMClient, error) {
	return f.RAMClient, nil
}
