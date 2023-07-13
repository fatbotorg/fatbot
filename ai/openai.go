package ai

import (
	"context"
	"fmt"
	"os"

	"github.com/charmbracelet/log"
	"github.com/getsentry/sentry-go"
	openai "github.com/sashabaranov/go-openai"
)

func GetAiResponse(labels []string) string {
	client := openai.NewClient(os.Getenv("OPENAI_APITOKEN"))
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Temperature: 1.0,
			Model:       openai.GPT3Dot5Turbo,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: fmt.Sprintf("Write a response to a user after their workout, congratulating them for their effort and enoucraging them to continue working out, address this list of words in your response: %s. Keep it under 240 characters. End the message with emojis matching the words from the list.", labels),
				},
			},
		},
	)

	if err != nil {
		log.Errorf("ChatCompletion error: %v\n", err)
		sentry.CaptureException(err)
		return ""
	}
	return resp.Choices[0].Message.Content
}
