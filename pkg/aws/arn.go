package aws

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws/arn"
)

func GetARNResourceID(resourceARN string) (string, error) {
	gatewayARN, err := arn.Parse(resourceARN)
	if err != nil {
		return "", fmt.Errorf("failed to parse arn: %w", err)
	}

	// The ARN struct holds the resource in the format "<resource-type>/<resource-name>"
	resourceSplit := strings.Split(gatewayARN.Resource, "/")
	return resourceSplit[1], nil
}
