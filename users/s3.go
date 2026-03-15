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

// UploadFileToS3 streams a file from disk directly to S3 without reading it
// fully into memory first. Prefer this over UploadToS3 for large files.
func UploadFileToS3(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	// Sniff content type from first 512 bytes without buffering the whole file
	header := make([]byte, 512)
	n, err := f.Read(header)
	if err != nil && err != io.EOF {
		return "", err
	}
	contentType := http.DetectContentType(header[:n])
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return "", err
	}

	bucket := viper.GetString("s3.bucket")
	if bucket == "" {
		bucket = os.Getenv("S3_BUCKET")
	}
	region := viper.GetString("s3.region")
	if region == "" {
		region = os.Getenv("AWS_REGION")
	}
	regionExplicit := region != ""
	if !regionExplicit {
		region = "us-east-1"
	}

	if bucket == "" {
		return "", fmt.Errorf("S3 bucket not configured")
	}

	sess, err := session.NewSession(&aws.Config{Region: aws.String(region)})
	if err != nil {
		return "", err
	}
	svc := s3.New(sess)

	key := filePath // use path as key (basename would also work)
	input := &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        f,
		ContentType: aws.String(contentType),
		ACL:         aws.String("public-read"),
	}
	_, err = svc.PutObject(input)
	if err != nil {
		log.Warn("S3 upload with public-read ACL failed, retrying without ACL", "error", err)
		if _, err2 := f.Seek(0, io.SeekStart); err2 != nil {
			return "", err2
		}
		input.ACL = nil
		if _, err = svc.PutObject(input); err != nil {
			return "", err
		}
	}

	if !regionExplicit {
		return fmt.Sprintf("https://s3.amazonaws.com/%s/%s", bucket, key), nil
	}
	return fmt.Sprintf("https://s3.%s.amazonaws.com/%s/%s", region, bucket, key), nil
}

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
