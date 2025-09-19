package aws

import (
	"bytes"
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
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
