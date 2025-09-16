package aws

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ram"
	"github.com/aws/aws-sdk-go-v2/service/route53resolver"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	awsv1 "github.com/aws/aws-sdk-go/aws"
	stscredsv1 "github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/request"
	awssession "github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/smithy-go/metrics/smithyotelmetrics"
	"github.com/pkg/errors"

	"github.com/aws-resolver-rules-operator/pkg/resolver"
)

const defaultSessionKey = "default"

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
	appID = fmt.Sprintf("%s/%s", controllerName, currentCommit)
)

func NewClients(endpoint string) *Clients {
	return &Clients{endpoint: endpoint}
}

func (c *Clients) NewResolverClient(region, roleToAssume string) (resolver.ResolverClient, error) {
	conf, err := c.configFromRegion(region)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	client := route53resolver.NewFromConfig(conf, func(o *route53resolver.Options) {
		o.Credentials = aws.NewCredentialsCache(
			stscreds.NewAssumeRoleProvider(sts.NewFromConfig(conf), roleToAssume),
		)
		o.AppID = appID
		o.MeterProvider = smithyotelmetrics.Adapt(metricProvider)
	})

	return &AWSResolver{client: client}, nil
}

func (c *Clients) NewResolverClientWithExternalId(region, roleToAssume, externalRoleToAssume, externalId string) (resolver.ResolverClient, error) {
	conf, err := config.LoadDefaultConfig(context.Background(),
		config.WithCredentialsProvider(aws.NewCredentialsCache(
			stscreds.NewAssumeRoleProvider(sts.New(sts.Options{}), roleToAssume),
		)),
	)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	client := route53resolver.NewFromConfig(conf, func(o *route53resolver.Options) {
		o.Credentials = aws.NewCredentialsCache(
			stscreds.NewAssumeRoleProvider(sts.NewFromConfig(conf), externalRoleToAssume, func(aro *stscreds.AssumeRoleOptions) {
				aro.ExternalID = aws.String(externalId)
			}),
		)
		o.AppID = appID
		o.MeterProvider = smithyotelmetrics.Adapt(metricProvider)
	})

	return &AWSResolver{client: client}, nil
}

func (c *Clients) NewEC2Client(region, arn string) (resolver.EC2Client, error) {
	client, err := c.newEC2Client(region, arn, "")
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &AWSEC2{client: client}, nil
}

func (c *Clients) NewEC2ClientWithExternalId(region, arn, externalId string) (resolver.EC2Client, error) {
	client, err := c.newEC2Client(region, arn, externalId)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &AWSEC2{client: client}, nil
}

func (c *Clients) newEC2Client(region, roleArn, externalId string) (*ec2.Client, error) {
	conf, err := c.configFromRegion(region)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	ec2Client := ec2.NewFromConfig(conf, func(o *ec2.Options) {
		o.Credentials = aws.NewCredentialsCache(
			stscreds.NewAssumeRoleProvider(sts.NewFromConfig(conf), roleArn, func(aro *stscreds.AssumeRoleOptions) {
				aro.ExternalID = aws.String(externalId)
			}),
		)
		o.AppID = appID
		o.MeterProvider = smithyotelmetrics.Adapt(metricProvider)
	})

	return ec2Client, nil
}

func (c *Clients) NewRAMClient(region, arn string) (resolver.RAMClient, error) {
	client, err := c.newRAMClient(region, arn, "")
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &RAM{client: client}, nil
}

func (c *Clients) NewRAMClientWithExternalId(region, arn, externalId string) (resolver.RAMClient, error) {
	client, err := c.newRAMClient(region, arn, externalId)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &RAM{client: client}, nil
}

func (c *Clients) newRAMClient(region, roleArn, externalId string) (*ram.Client, error) {
	conf, err := c.configFromRegion(region)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	ramClient := ram.NewFromConfig(conf, func(o *ram.Options) {
		o.Credentials = aws.NewCredentialsCache(
			stscreds.NewAssumeRoleProvider(sts.NewFromConfig(conf), roleArn, func(aro *stscreds.AssumeRoleOptions) {
				aro.ExternalID = aws.String(externalId)
			}),
		)
		o.AppID = appID
		o.MeterProvider = smithyotelmetrics.Adapt(metricProvider)
	})

	return ramClient, nil
}

func (c *Clients) NewRoute53Client(region, arn string) (resolver.Route53Client, error) {
	client, err := c.newRoute53Client(region, arn, "")
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return NewRoute53(client), nil
}

func (c *Clients) NewS3Client(region, arn string) (resolver.S3Client, error) {
	client, err := c.newS3Client(region, arn, "")
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &S3Client{client: client}, nil
}

func (c *Clients) NewTransitGatewayClient(region, rolearn string) (resolver.TransitGatewayClient, error) {
	conf, err := c.configForRole(rolearn)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	ec2Client := ec2.NewFromConfig(conf, func(o *ec2.Options) {
		o.Region = region
	})

	return &TransitGateways{
		ec2: ec2Client,
	}, nil
}

func (c *Clients) NewPrefixListClient(region, rolearn string) (resolver.PrefixListClient, error) {
	conf, err := c.configForRole(rolearn)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	ec2Client := ec2.NewFromConfig(conf, func(o *ec2.Options) {
		o.Region = region
	})

	return &PrefixLists{
		client: ec2Client,
	}, nil
}

func (c *Clients) NewRouteTableClient(region, rolearn string) (resolver.RouteTableClient, error) {
	conf, err := c.configForRole(rolearn)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	ec2Client := ec2.NewFromConfig(conf, func(o *ec2.Options) {
		o.Region = region
	})

	return &RouteTableClient{
		client: ec2Client,
	}, nil
}

func (c *Clients) newRoute53Client(region, arn, externalId string) (*route53.Route53, error) {
	session, err := c.sessionFromRegion(region)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	route53Client := route53.New(session, &awsv1.Config{Credentials: stscredsv1.NewCredentials(session, arn, configureExternalId(arn, externalId))})
	route53Client.Handlers.Build.PushFront(request.MakeAddToUserAgentHandler(controllerName, currentCommit))
	route53Client.Handlers.CompleteAttempt.PushFront(captureRequestMetrics(controllerName))

	return route53Client, nil
}

func (c *Clients) newS3Client(region, arn, externalId string) (*s3.S3, error) {
	session, err := c.sessionFromRegion(region)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	s3Client := s3.New(session, &awsv1.Config{Credentials: stscredsv1.NewCredentials(session, arn, configureExternalId(arn, externalId))})
	s3Client.Handlers.Build.PushFront(request.MakeAddToUserAgentHandler(controllerName, currentCommit))
	s3Client.Handlers.CompleteAttempt.PushFront(captureRequestMetrics(controllerName))

	return s3Client, nil
}

func configureExternalId(roleArn, externalId string) func(provider *stscredsv1.AssumeRoleProvider) {
	return func(assumeRoleProvider *stscredsv1.AssumeRoleProvider) {
		if roleArn != "" {
			assumeRoleProvider.RoleARN = roleArn
		}
		if externalId != "" {
			assumeRoleProvider.ExternalID = aws.String(externalId)
		}
	}
}

func (c *Clients) config() (aws.Config, error) {
	if conf, ok := configCache.Load(defaultSessionKey); ok {
		entry := conf.(*configCacheEntry)
		return entry.config, nil
	}

	conf, err := config.LoadDefaultConfig(context.TODO(),
		config.WithBaseEndpoint(c.endpoint),
	)
	if err != nil {
		return emptyConfig, errors.WithStack(err)
	}

	configCache.Store(defaultSessionKey, &configCacheEntry{
		config: conf,
	})
	return conf, nil
}

func (c *Clients) configForRole(roleArn string) (aws.Config, error) {
	if conf, ok := configCache.Load(roleArn); ok {
		entry := conf.(*configCacheEntry)
		return entry.config, nil
	}

	defaultConfig, err := c.config()
	if err != nil {
		return emptyConfig, errors.WithStack(err)
	}

	creds := stscreds.NewAssumeRoleProvider(sts.NewFromConfig(defaultConfig), roleArn)

	conf := defaultConfig.Copy()
	conf.Credentials = aws.NewCredentialsCache(creds)

	configCache.Store(roleArn, &configCacheEntry{
		config: conf,
	})
	return conf, nil
}

func (c *Clients) configFromRegion(region string) (aws.Config, error) {
	if conf, ok := configCache.Load(region); ok {
		entry := conf.(*configCacheEntry)
		return entry.config, nil
	}

	conf, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(region),
		config.WithBaseEndpoint(c.endpoint),
	)
	if err != nil {
		return emptyConfig, errors.WithStack(err)
	}

	configCache.Store(region, &configCacheEntry{
		config: conf,
	})
	return conf, nil
}

var emptyConfig = aws.Config{}

var configCache sync.Map

type configCacheEntry struct {
	config aws.Config
}

func (c *Clients) session() (*awssession.Session, error) {
	if s, ok := sessionCache.Load(defaultSessionKey); ok {
		entry := s.(*sessionCacheEntry)
		return entry.session, nil
	}

	ns, err := awssession.NewSession(&awsv1.Config{
		Endpoint: aws.String(c.endpoint),
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	sessionCache.Store(defaultSessionKey, &sessionCacheEntry{
		session: ns,
	})
	return ns, nil
}

func (c *Clients) sessionForRole(roleArn string) (*awssession.Session, error) {
	if s, ok := sessionCache.Load(roleArn); ok {
		entry := s.(*sessionCacheEntry)
		return entry.session, nil
	}

	defaultSession, err := c.session()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	ns, err := awssession.NewSession(&awsv1.Config{
		Endpoint:    awsv1.String(c.endpoint),
		Credentials: stscredsv1.NewCredentials(defaultSession, roleArn),
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	sessionCache.Store(roleArn, &sessionCacheEntry{
		session: ns,
	})
	return ns, nil
}

func (c *Clients) sessionFromRegion(region string) (*awssession.Session, error) {
	if s, ok := sessionCache.Load(region); ok {
		entry := s.(*sessionCacheEntry)
		return entry.session, nil
	}

	ns, err := awssession.NewSession(&awsv1.Config{
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
