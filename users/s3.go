package users

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/charmbracelet/log"
	"github.com/spf13/viper"
)

func UploadToS3(fileName string, fileBytes []byte) (string, error) {
	bucket := viper.GetString("s3.bucket")
	if bucket == "" {
		bucket = os.Getenv("S3_BUCKET")
	}
	region := viper.GetString("s3.region")
	if region == "" {
		region = os.Getenv("AWS_REGION")
	}
	if region == "" {
		region = "us-east-1"
	}

	log.Debug("Uploading to S3", "bucket", bucket, "region", region, "fileName", fileName)

	if bucket == "" {
		return "", fmt.Errorf("S3 bucket not configured")
	}

	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(region),
	})
	if err != nil {
		return "", err
	}

	svc := s3.New(sess)

	bodyReader := bytes.NewReader(fileBytes)
	input := &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(fileName),
		Body:        bodyReader,
		ContentType: aws.String(http.DetectContentType(fileBytes)),
		ACL:         aws.String("public-read"), // Instagram needs access
	}

	_, err = svc.PutObject(input)
	if err != nil {
		// If ACLs are disabled, try without ACL
		log.Warn("S3 upload with public-read ACL failed, retrying without ACL. Note: Instagram publication will fail if the bucket/object is not public!", "error", err)
		// Reset the reader to the beginning
		bodyReader.Seek(0, io.SeekStart)
		input.ACL = nil
		_, err = svc.PutObject(input)
		if err != nil {
			return "", err
		}
	}

	if region == "" {
		return fmt.Sprintf("https://s3.amazonaws.com/%s/%s", bucket, fileName), nil
	}
	return fmt.Sprintf("https://s3.%s.amazonaws.com/%s/%s", region, bucket, fileName), nil
}
