package awsclient

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	awssession "github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/route53resolver"

	"github.com/aws-resolver-rules-operator/controllers"
)

type ClientsFactory struct {
}

func (c *ClientsFactory) NewRoute53ResolverClient(session *awssession.Session, arn string) controllers.Route53ResolverClient {
	return &AWSRoute53Resolver{Route53resolverClient: route53resolver.New(session, &aws.Config{Credentials: stscreds.NewCredentials(session, arn)})}
}

func (c *ClientsFactory) NewEC2Client(session *awssession.Session, arn string) controllers.EC2Client {
	return &AWSEC2{EC2Client: ec2.New(session, &aws.Config{Credentials: stscreds.NewCredentials(session, arn)})}
}
