package aws_test

import (
	"context"
	"errors"
	"os"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/aws-resolver-rules-operator/pkg/resolver"
)

var _ = Describe("S3 client", func() {
	var (
		ctx context.Context

		rawS3Client *s3.Client
		s3Client    resolver.S3Client
		bucket      string
		key         string
	)

	objectExists := func() bool {
		_, err := rawS3Client.HeadObject(ctx, &s3.HeadObjectInput{
			Bucket: awssdk.String(bucket),
			Key:    awssdk.String(key),
		})
		if err == nil {
			return true
		}
		var notFound *s3types.NotFound
		Expect(errors.As(err, &notFound)).To(BeTrue(), "unexpected error: %v", err)
		return false
	}

	BeforeEach(func() {
		ctx = context.Background()
		bucket = uuid.NewString()
		key = "karpenter-machine-pool/" + uuid.NewString()

		cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(Region))
		Expect(err).NotTo(HaveOccurred())
		rawS3Client = s3.NewFromConfig(cfg, func(o *s3.Options) {
			o.BaseEndpoint = awssdk.String(os.Getenv("AWS_ENDPOINT"))
			o.UsePathStyle = true
		})

		_, err = rawS3Client.CreateBucket(ctx, &s3.CreateBucketInput{
			Bucket: awssdk.String(bucket),
			CreateBucketConfiguration: &s3types.CreateBucketConfiguration{
				LocationConstraint: s3types.BucketLocationConstraint(Region),
			},
		})
		Expect(err).NotTo(HaveOccurred())

		s3Client, err = awsClients.NewS3Client(Region, AwsIamArn)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Delete", func() {
		It("removes an object written by Put", func() {
			Expect(s3Client.Put(ctx, bucket, key, []byte("userdata"))).To(Succeed())
			Expect(objectExists()).To(BeTrue())

			Expect(s3Client.Delete(ctx, bucket, key)).To(Succeed())
			Expect(objectExists()).To(BeFalse())
		})

		It("succeeds when the object does not exist", func() {
			Expect(s3Client.Delete(ctx, bucket, key)).To(Succeed())
		})

		It("succeeds when the bucket does not exist", func() {
			Expect(s3Client.Delete(ctx, uuid.NewString(), key)).To(Succeed())
		})
	})
})
