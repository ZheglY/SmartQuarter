// devstorage initializes only the dedicated local MinIO bucket.
package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	"os"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("ru-central1"))
	if err != nil {
		return errors.New("cannot load local S3 credentials")
	}
	api := s3.NewFromConfig(cfg, func(o *s3.Options) { o.BaseEndpoint = aws.String("http://localhost:19000"); o.UsePathStyle = true })
	for attempt := 0; attempt < 20; attempt++ {
		_, err = api.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String("issue-test")})
		if err == nil {
			return nil
		}
		var ae smithy.APIError
		if errors.As(err, &ae) && (ae.ErrorCode() == "BucketAlreadyOwnedByYou" || ae.ErrorCode() == "BucketAlreadyExists") {
			return nil
		}
		select {
		case <-ctx.Done():
			return errors.New("local MinIO unavailable")
		case <-time.After(time.Second):
		}
	}
	return errors.New("cannot initialize local MinIO bucket")
}
