package aws

import (
	"bytes"
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"github.com/pkg/errors"
)

type S3Client struct {
	client *s3.Client
}

func (c S3Client) Put(ctx context.Context, bucket, path string, data []byte) error {
	_, err := c.client.PutObject(ctx, &s3.PutObjectInput{
		Body:                 bytes.NewReader(data),
		Bucket:               aws.String(bucket),
		Key:                  aws.String(path),
		ServerSideEncryption: types.ServerSideEncryptionAwsKms,
	})
	if err != nil {
		return err
	}
	return nil
}

// Delete removes the object at path. S3 answers a delete of a missing key with success, and a
// missing bucket leaves nothing to delete either, so both return nil.
func (c S3Client) Delete(ctx context.Context, bucket, path string) error {
	_, err := c.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(path),
	})
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && apiErr.ErrorCode() == "NoSuchBucket" {
			return nil
		}
		return errors.WithStack(err)
	}
	return nil
}
