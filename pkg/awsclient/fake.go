package awsclient

import (
	awssession "github.com/aws/aws-sdk-go/aws/session"

	"github.com/aws-resolver-rules-operator/controllers"
)

type FakeClientsFactory struct {
	Route53ResolverClient controllers.Route53ResolverClient
	EC2Client             controllers.EC2Client
}

func (f *FakeClientsFactory) NewRoute53ResolverClient(session *awssession.Session, arn string) controllers.Route53ResolverClient {
	return f.Route53ResolverClient
}

func (f *FakeClientsFactory) NewEC2Client(session *awssession.Session, arn string) controllers.EC2Client {
	return f.EC2Client
}
