package updates

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/rekognition"
	"github.com/charmbracelet/log"
	"github.com/getsentry/sentry-go"
)

func detectImageLabels(imageBytes []byte) []string {
	svc := rekognition.New(session.New())
	input := &rekognition.DetectLabelsInput{
		Image: &rekognition.Image{
			Bytes: imageBytes,
		},
		MaxLabels:     aws.Int64(50),
		MinConfidence: aws.Float64(80.000000),
	}

	result, err := svc.DetectLabels(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case rekognition.ErrCodeInvalidS3ObjectException:
				log.Error(rekognition.ErrCodeInvalidS3ObjectException, aerr.Error())
			case rekognition.ErrCodeInvalidParameterException:
				log.Error(rekognition.ErrCodeInvalidParameterException, aerr.Error())
			case rekognition.ErrCodeImageTooLargeException:
				log.Error(rekognition.ErrCodeImageTooLargeException, aerr.Error())
			case rekognition.ErrCodeAccessDeniedException:
				log.Error(rekognition.ErrCodeAccessDeniedException, aerr.Error())
			case rekognition.ErrCodeInternalServerError:
				log.Error(rekognition.ErrCodeInternalServerError, aerr.Error())
			case rekognition.ErrCodeThrottlingException:
				log.Error(rekognition.ErrCodeThrottlingException, aerr.Error())
			case rekognition.ErrCodeProvisionedThroughputExceededException:
				log.Error(rekognition.ErrCodeProvisionedThroughputExceededException, aerr.Error())
			case rekognition.ErrCodeInvalidImageFormatException:
				log.Error(rekognition.ErrCodeInvalidImageFormatException, aerr.Error())
			default:
				log.Error(aerr.Error())
			}
			sentry.CaptureException(err)
		} else {
			log.Error(err.Error())
		}
		return []string{}
	}

	answer := []string{}
	unwantedLabels := map[string]byte{
		"adult":  0,
		"male":   0,
		"female": 0,
	}
	for _, label := range result.Labels {
		if _, ok := unwantedLabels[strings.ToLower(*label.Name)]; ok {
			continue
		}
		answer = append(answer, *label.Name)
	}
	return answer
}

func findEmoji(label string) string {
	acceptedLables := map[string]string{
		"fitness":       "🤾",
		"working out":   "🏋️",
		"swimming pool": "🏊",
		"pilates":       "🧘",
		"yoga":          "🧘",
		"running":       "🏃",
		"weights":       "🏋️",
		"gym":           "🤸",
		"sport":         "💪",
		"swim":          "🏊",
		"run":           "🏃",
		"jog":           "🏃",
		"sweat":         "💦",
		"climbing":      "🧗",
		"goggles":       "🥽",
		"cycling":       "🚴",
		"bicycle":       "🚴",
		"football":      "⚽",
		"soccer":        "⚽",
		"basketball":    "🏀",
		"tennis":        "🎾",
	}
	emoji, ok := acceptedLables[strings.ToLower(label)]
	if ok {
		return fmt.Sprintf("%s:%s", label, emoji)
	}
	return ""
}
