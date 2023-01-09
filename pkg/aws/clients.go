package aws

import (
	"runtime/debug"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/request"
	awssession "github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ram"
	"github.com/aws/aws-sdk-go/service/route53resolver"
	"github.com/pkg/errors"

	"github.com/aws-resolver-rules-operator/pkg/resolver"
)

type Clients struct {
	// endpoint is the AWS API endpoint to use
	endpoint string
}

var (
	controllerName = "aws-resolver-rules-operator"
	currentCommit  = func() string {
		if info, ok := debug.ReadBuildInfo(); ok {
			for _, setting := range info.Settings {
				if setting.Key == "vcs.revision" {
					return setting.Value
				}
			}
		}

		return ""
	}()
)

func NewClients(endpoint string) resolver.AWSClients {
	return &Clients{endpoint: endpoint}
}

func (c *Clients) NewResolverClient(region, arn string) (resolver.ResolverClient, error) {
	client, err := c.newResolverClient(region, arn, "")
	if err != nil {
		return &AWSResolver{}, errors.WithStack(err)
	}

	return &AWSResolver{client: client}, nil
}

func (c *Clients) NewResolverClientWithExternalId(region, arn, externalId string) (resolver.ResolverClient, error) {
	client, err := c.newResolverClient(region, arn, externalId)
	if err != nil {
		return &AWSResolver{}, errors.WithStack(err)
	}

	return &AWSResolver{client: client}, nil
}

func (c *Clients) newResolverClient(region, arn, externalId string) (*route53resolver.Route53Resolver, error) {
	session, err := c.sessionFromRegion(region)
	if err != nil {
		return &route53resolver.Route53Resolver{}, errors.WithStack(err)
	}

	resolverClient := route53resolver.New(session, &aws.Config{Credentials: stscreds.NewCredentials(session, arn, configureExternalId(externalId))})
	resolverClient.Handlers.Build.PushFront(request.MakeAddToUserAgentHandler(controllerName, currentCommit))
	resolverClient.Handlers.CompleteAttempt.PushFront(captureRequestMetrics(controllerName))

	return resolverClient, nil
}

func (c *Clients) NewEC2Client(region, arn string) (resolver.EC2Client, error) {
	client, err := c.newEC2Client(region, arn, "")
	if err != nil {
		return &AWSEC2{}, errors.WithStack(err)
	}

	return &AWSEC2{client: client}, nil
}

func (c *Clients) NewEC2ClientWithExternalId(region, arn, externalId string) (resolver.EC2Client, error) {
	client, err := c.newEC2Client(region, arn, externalId)
	if err != nil {
		return &AWSEC2{}, errors.WithStack(err)
	}

	return &AWSEC2{client: client}, nil
}

func (c *Clients) newEC2Client(region, arn, externalId string) (*ec2.EC2, error) {
	session, err := c.sessionFromRegion(region)
	if err != nil {
		return &ec2.EC2{}, errors.WithStack(err)
	}

	ec2Client := ec2.New(session, &aws.Config{Credentials: stscreds.NewCredentials(session, arn, configureExternalId(externalId))})
	ec2Client.Handlers.Build.PushFront(request.MakeAddToUserAgentHandler(controllerName, currentCommit))
	ec2Client.Handlers.CompleteAttempt.PushFront(captureRequestMetrics(controllerName))

	return ec2Client, nil
}

func (c *Clients) NewRAMClient(region, arn string) (resolver.RAMClient, error) {
	client, err := c.newRAMClient(region, arn, "")
	if err != nil {
		return &AWSRAM{}, errors.WithStack(err)
	}

	return &AWSRAM{client: client}, nil
}

func (c *Clients) NewRAMClientWithExternalId(region, arn, externalId string) (resolver.RAMClient, error) {
	client, err := c.newRAMClient(region, arn, externalId)
	if err != nil {
		return &AWSRAM{}, errors.WithStack(err)
	}

	return &AWSRAM{client: client}, nil
}

func (c *Clients) newRAMClient(region, arn, externalId string) (*ram.RAM, error) {
	session, err := c.sessionFromRegion(region)
	if err != nil {
		return &ram.RAM{}, errors.WithStack(err)
	}

	ramClient := ram.New(session, &aws.Config{Credentials: stscreds.NewCredentials(session, arn, configureExternalId(externalId))})
	ramClient.Handlers.Build.PushFront(request.MakeAddToUserAgentHandler(controllerName, currentCommit))
	ramClient.Handlers.CompleteAttempt.PushFront(captureRequestMetrics(controllerName))

	return ramClient, nil
}

func configureExternalId(externalId string) func(provider *stscreds.AssumeRoleProvider) {
	return func(assumeRoleProvider *stscreds.AssumeRoleProvider) {
		if externalId != "" {
			assumeRoleProvider.ExternalID = aws.String(externalId)
		}
	}
}

func (c *Clients) sessionFromRegion(region string) (*awssession.Session, error) {
	if s, ok := sessionCache.Load(region); ok {
		entry := s.(*sessionCacheEntry)
		return entry.session, nil
	}

	ns, err := awssession.NewSession(&aws.Config{
		Region:   aws.String(region),
		Endpoint: aws.String(c.endpoint),
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	sessionCache.Store(region, &sessionCacheEntry{
		session: ns,
	})
	return ns, nil
}

var sessionCache sync.Map

type sessionCacheEntry struct {
	session *awssession.Session
}
