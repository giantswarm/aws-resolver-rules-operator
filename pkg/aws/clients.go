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

func (c *Clients) NewResolverClient(region, arn, externalId string) (resolver.ResolverClient, error) {
	session, err := sessionFromRegion(region)
	if err != nil {
		return &AWSResolver{}, errors.WithStack(err)
	}

	resolverClient := route53resolver.New(session, &aws.Config{Credentials: stscreds.NewCredentials(session, arn, configureExternalId(externalId))})
	resolverClient.Handlers.Build.PushFront(request.MakeAddToUserAgentHandler(controllerName, currentCommit))
	resolverClient.Handlers.CompleteAttempt.PushFront(captureRequestMetrics(controllerName))

	return &AWSResolver{client: resolverClient}, nil
}

func (c *Clients) NewEC2Client(region, arn string) (resolver.EC2Client, error) {
	session, err := sessionFromRegion(region)
	if err != nil {
		return &AWSEC2{}, errors.WithStack(err)
	}

	ec2Client := ec2.New(session, &aws.Config{Credentials: stscreds.NewCredentials(session, arn)})
	ec2Client.Handlers.Build.PushFront(request.MakeAddToUserAgentHandler(controllerName, currentCommit))
	ec2Client.Handlers.CompleteAttempt.PushFront(captureRequestMetrics(controllerName))

	return &AWSEC2{client: ec2Client}, nil
}

func (c *Clients) NewRAMClient(region, arn string) (resolver.RAMClient, error) {
	session, err := sessionFromRegion(region)
	if err != nil {
		return &AWSRAM{}, errors.WithStack(err)
	}

	ramClient := ram.New(session, &aws.Config{Credentials: stscreds.NewCredentials(session, arn)})
	ramClient.Handlers.Build.PushFront(request.MakeAddToUserAgentHandler(controllerName, currentCommit))
	ramClient.Handlers.CompleteAttempt.PushFront(captureRequestMetrics(controllerName))

	return &AWSRAM{client: ramClient}, nil
}

func configureExternalId(externalId string) func(provider *stscreds.AssumeRoleProvider) {
	return func(assumeRoleProvider *stscreds.AssumeRoleProvider) {
		if externalId != "" {
			assumeRoleProvider.ExternalID = aws.String(externalId)
		}
	}
}

func sessionFromRegion(region string) (*awssession.Session, error) {
	if s, ok := sessionCache.Load(region); ok {
		entry := s.(*sessionCacheEntry)
		return entry.session, nil
	}

	ns, err := awssession.NewSession(&aws.Config{
		Region: aws.String(region),
	})
	if err != nil {
		return nil, err
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
