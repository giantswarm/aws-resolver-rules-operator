package aws

import (
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	awssession "github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ram"
	"github.com/aws/aws-sdk-go/service/route53resolver"
	"github.com/pkg/errors"

	"github.com/aws-resolver-rules-operator/pkg/aws/clients"
)

type Clients struct {
}

func (c *Clients) NewResolverClient(region, arn, externalId string) (ResolverClient, error) {
	session, err := sessionFromRegion(region)
	if err != nil {
		return &clients.AWSResolver{}, errors.WithStack(err)
	}

	return &clients.AWSResolver{ResolverClient: route53resolver.New(session, &aws.Config{Credentials: stscreds.NewCredentials(session, arn, configureExternalId(externalId))})}, nil
}

func (c *Clients) NewEC2Client(region, arn string) (EC2Client, error) {
	session, err := sessionFromRegion(region)
	if err != nil {
		return &clients.AWSEC2{}, errors.WithStack(err)
	}

	return &clients.AWSEC2{EC2Client: ec2.New(session, &aws.Config{Credentials: stscreds.NewCredentials(session, arn)})}, nil
}

func (c *Clients) NewRAMClient(region, arn string) (RAMClient, error) {
	session, err := sessionFromRegion(region)
	if err != nil {
		return &clients.AWSRAM{}, errors.WithStack(err)
	}

	return &clients.AWSRAM{RAMClient: ram.New(session, &aws.Config{Credentials: stscreds.NewCredentials(session, arn)})}, nil
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
