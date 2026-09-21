package yandexs3

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/domain"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/storage"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

type Options struct {
	Endpoint, PublicEndpoint, Region, Bucket string
	PathStyle                                bool
}
type Client struct {
	api     *s3.Client
	presign *s3.PresignClient
	bucket  string
	observe func(string, error)
}

func New(ctx context.Context, o Options, observe func(string, error)) (*Client, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(o.Region), awsconfig.WithHTTPClient(&http.Client{Timeout: 20 * time.Second}))
	if err != nil {
		return nil, domain.ErrUnavailable
	}
	cfg.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
	cfg.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
	makeClient := func(endpoint string) *s3.Client {
		return s3.NewFromConfig(cfg, func(opt *s3.Options) {
			opt.BaseEndpoint = aws.String(endpoint)
			opt.UsePathStyle = o.PathStyle
			opt.RetryMaxAttempts = 2
		})
	}
	api := makeClient(o.Endpoint)
	public := api
	if o.PublicEndpoint != "" && o.PublicEndpoint != o.Endpoint {
		public = makeClient(o.PublicEndpoint)
	}
	if observe == nil {
		observe = func(string, error) {}
	}
	return &Client{api: api, presign: s3.NewPresignClient(public), bucket: o.Bucket, observe: observe}, nil
}
func translate(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NoSuchKey", "NotFound":
			return storage.ErrObjectNotFound
		case "NoSuchBucket":
			return domain.ErrUnavailable
		case "PreconditionFailed":
			return domain.ErrPrecondition
		}
	}
	return domain.ErrUnavailable
}
func (c *Client) Check(ctx context.Context) (err error) {
	defer func() { c.observe("head_bucket", err) }()
	_, err = c.api.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(c.bucket)})
	return translate(err)
}
