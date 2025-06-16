package aws

import (
	"bytes"
	"context"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
)

type S3Client struct {
	client *s3.S3
}

func (c S3Client) Put(ctx context.Context, bucket, path string, data []byte) error {
	_, err := c.client.PutObjectWithContext(ctx, &s3.PutObjectInput{
		Body:                 aws.ReadSeekCloser(bytes.NewReader(data)),
		Bucket:               aws.String(bucket),
		Key:                  aws.String(path),
		ServerSideEncryption: aws.String("aws:kms"),
	})
	if err != nil {
		return err
	}
	return nil
}
