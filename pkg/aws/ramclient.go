package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/ram"
	"github.com/aws/aws-sdk-go-v2/service/ram/types"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/aws-resolver-rules-operator/pkg/resolver"
)

type RAM struct {
	client *ram.Client
}

func (c *RAM) ApplyResourceShare(ctx context.Context, share resolver.ResourceShare) error {
	logger := c.getLogger(ctx)
	logger = logger.WithValues(
		"resource-share-name", share.Name,
		"resource-arns", share.ResourceArns,
		"external-accound-id", share.ExternalAccountID,
	)
	resourceARN, err := arn.Parse(share.ResourceArns[0])
	if err != nil {
		logger.Error(err, "failed to parse transit gateway arn")
		return errors.WithStack(err)
	}

	if share.ExternalAccountID == resourceARN.AccountID {
		logger.Info("resource in same account as cluster, there is no need to share it using ram. Skipping")
		return nil
	}

	resourceShare, err := c.getResourceShare(ctx, share.Name)
	if err != nil {
		logger.Error(err, "failed to get resource share")
		return errors.WithStack(err)
	}

	if resourceShare != nil {
		logger.Info("resource share already exists")
		return nil
	}

	logger.Info("creating resource share")
	_, err = c.client.CreateResourceShare(ctx, &ram.CreateResourceShareInput{
		AllowExternalPrincipals: aws.Bool(true),
		Name:                    aws.String(share.Name),
		Principals:              []string{share.ExternalAccountID},
		ResourceArns:            share.ResourceArns,
	})
	if err != nil {
		logger.Error(err, "failed to create resource share")
		return errors.WithStack(err)
	}

	return nil
}

func (c *RAM) DeleteResourceShare(ctx context.Context, name string) error {
	logger := c.getLogger(ctx)
	logger = logger.WithValues("resource-share-name", name)

	resourceShare, err := c.getResourceShare(ctx, name)
	if err != nil {
		logger.Error(err, "failed to get resource share")
		return err
	}

	if resourceShare == nil {
		logger.Info("resource share not found")
		return nil
	}

	_, err = c.client.DeleteResourceShare(ctx, &ram.DeleteResourceShareInput{
		ResourceShareArn: resourceShare.ResourceShareArn,
	})
	return err
}

func (c *RAM) getResourceShare(ctx context.Context, name string) (*types.ResourceShare, error) {
	logger := c.getLogger(ctx)
	logger = logger.WithValues("resource-share-name", name)

	resourceShareOutput, err := c.client.GetResourceShares(ctx, &ram.GetResourceSharesInput{
		Name:          aws.String(name),
		ResourceOwner: types.ResourceOwnerSelf,
	})
	if err != nil {
		logger.Error(err, "failed to get resource share")
		return nil, errors.WithStack(err)
	}

	resourceShares := filterDeletedResourceShares(resourceShareOutput.ResourceShares)

	if len(resourceShares) == 0 {
		logger.Info("no resource shares found")
		return nil, nil
	}

	if len(resourceShares) > 1 {
		err = fmt.Errorf("expected 1 resource share, found %d", len(resourceShares))
		logger.Error(err, "wrong number of resource shares")
		return nil, err
	}

	return &resourceShares[0], nil
}

func (c *RAM) getLogger(ctx context.Context) logr.Logger {
	logger := log.FromContext(ctx)
	logger = logger.WithName("ram-client")
	return logger
}

func filterDeletedResourceShares(resourceShares []types.ResourceShare) []types.ResourceShare {
	filtered := []types.ResourceShare{}
	for _, share := range resourceShares {
		if !isResourceShareDeleted(share) {
			filtered = append(filtered, share)
		}
	}

	return filtered
}

func isResourceShareDeleted(resourceShare types.ResourceShare) bool {
	status := resourceShare.Status
	return status == types.ResourceShareStatusDeleted || status == types.ResourceShareStatusDeleting
}
