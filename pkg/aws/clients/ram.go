package clients

import (
	"context"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ram"
	"github.com/pkg/errors"
)

type AWSRAM struct {
	RAMClient *ram.RAM
}

func (a *AWSRAM) CreateResourceShareWithContext(ctx context.Context, resourceShareName string, allowExternalPrincipals bool, resourceArns, principals []string) (string, error) {
	response, err := a.RAMClient.CreateResourceShareWithContext(ctx, &ram.CreateResourceShareInput{
		AllowExternalPrincipals: aws.Bool(allowExternalPrincipals),
		Name:                    aws.String(resourceShareName),
		Principals:              aws.StringSlice(principals),
		ResourceArns:            aws.StringSlice(resourceArns),
	})
	if err != nil {
		return "", errors.WithStack(err)
	}

	return *response.ResourceShare.ResourceShareArn, nil
}
