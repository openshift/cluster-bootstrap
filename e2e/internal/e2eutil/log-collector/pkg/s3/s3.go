package s3

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

// Config defines configuration for s3 output.
type Config struct {
	AccessKeyId     string
	AccessKeySecret string
	BucketName      string
	BucketPrefix    string
	Region          string
}

// S3 implements Collector.Ouput Interface.
// S3 allows the logs to be uploaded to S3 bucket with defined prefix.
type S3 struct {
	client       *s3manager.Uploader
	bucketName   string
	bucketPrefix string
}

// New returns *S3.
// It performs an authentication of the specified credentials and returns error if failed.
func New(config *Config) (*S3, error) {
	creds := credentials.NewStaticCredentials(config.AccessKeyId, config.AccessKeySecret, "")
	var err error
	_, err = creds.Get()
	if err != nil {
		return nil, err
	}

	cfg := aws.NewConfig().WithRegion(config.Region).WithCredentials(creds).WithEndpoint("s3.amazonaws.com").WithS3ForcePathStyle(true)
	svc := s3.New(session.New(), cfg)

	uploader := s3manager.NewUploaderWithClient(svc, func(u *s3manager.Uploader) {
		u.LeavePartsOnError = true
		u.PartSize = 10 * 1024 * 1024
	})
	return &S3{
		client:       uploader,
		bucketName:   config.BucketName,
		bucketPrefix: config.BucketPrefix,
	}, nil
}

// Put uploads the data from f io.ReadSeeker to S3 bucket with bucketprefix.
// Put uses s3manager to upload large files in parts concurrently.
// Put returns the URL of the file in the bucket (the url can be used to access the file only if the bucket permissions allow).
func (s *S3) Put(f io.ReadSeeker, dst string) (string, error) {
	path := filepath.Join(s.bucketPrefix, dst)
	params := &s3manager.UploadInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(path),

		Body: f,
	}

	resp, err := s.client.Upload(params)
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			return "", fmt.Errorf("Code: %s Message: %s Orig: %v", awsErr.Code(), awsErr.Message(), awsErr.OrigErr())
		}
		return "", err
	}

	return resp.Location, nil
}
