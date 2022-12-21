package aws

import (
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	awssession "github.com/aws/aws-sdk-go/aws/session"
)

func SessionFromRegion(region string) (*awssession.Session, error) {
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
