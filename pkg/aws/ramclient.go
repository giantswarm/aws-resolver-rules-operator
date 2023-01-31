package aws

import (
	"context"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ram"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
)

type RAM struct {
	client *ram.RAM
}

// CreateResourceShareWithContext will create a RAM resource share to share an AWS resource (`resourceArn`) with another
// AWS account (`principal`).
// It will try to find a resource share with the same name. If it finds it, it will return its ARN, and it won't create
// a new RAM resource share.
func (a *RAM) CreateResourceShareWithContext(ctx context.Context, logger logr.Logger, resourceShareName, resourceArn, principal string) (string, error) {
	logger = logger.WithValues("resourceShareName", resourceShareName, "sharedWith", principal, "sharedResourceArn", resourceArn)
	logger = logger.WithValues("resourceShareName", resourceShareName, "sharedWith", principal, "sharedResourceArn", resourceArn)

	logger.Info("Trying to find RAM resource share by name")
	getResourceSharesResponse, err := a.client.GetResourceSharesWithContext(ctx, &ram.GetResourceSharesInput{
		Name:                aws.String(resourceShareName),
		ResourceOwner:       aws.String("SELF"),
		ResourceShareStatus: aws.String(ram.ResourceShareStatusActive),
	})
	if err != nil {
		return "", errors.WithStack(err)
	}

	if len(getResourceSharesResponse.ResourceShares) > 0 {
		logger.Info("RAM resource share was already there, skipping creation", "resourceShareArn", *getResourceSharesResponse.ResourceShares[0].ResourceShareArn, "owningAccountId", *getResourceSharesResponse.ResourceShares[0].OwningAccountId)
		return *getResourceSharesResponse.ResourceShares[0].ResourceShareArn, nil
	}

	logger.Info("Resource share was not found. Creating it so we can share Resolver Rule with a different AWS account")
	response, err := a.client.CreateResourceShareWithContext(ctx, &ram.CreateResourceShareInput{
		Name:         aws.String(resourceShareName),
		Principals:   aws.StringSlice([]string{principal}),
		ResourceArns: aws.StringSlice([]string{resourceArn}),
	})
	if err != nil {
		return "", errors.WithStack(err)
	}

	logger.Info("Resource share created")

	return *response.ResourceShare.ResourceShareArn, nil
}

func (a *RAM) DeleteResourceShareWithContext(ctx context.Context, logger logr.Logger, resourceShareName string) error {
	logger = logger.WithValues("resourceShareName", resourceShareName)

	logger.Info("Trying to find RAM resource share to delete")
	resourceShare, err := a.client.GetResourceSharesWithContext(ctx, &ram.GetResourceSharesInput{
		Name:          aws.String(resourceShareName),
		ResourceOwner: aws.String("SELF"),
	})
	if err != nil {
		return errors.WithStack(err)
	}

	if len(resourceShare.ResourceShares) < 1 {
		logger.Info("It did not find any RAM resource share using this name, skipping deletion")
		return nil
	}

	logger.Info("Deleting RAM resource share")
	_, err = a.client.DeleteResourceShareWithContext(ctx, &ram.DeleteResourceShareInput{
		ResourceShareArn: resourceShare.ResourceShares[0].ResourceShareArn,
	})
	if err != nil {
		return errors.WithStack(err)
	}

	logger.Info("Deleted RAM resource share")

	return nil
}
