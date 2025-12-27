package ai

import (
	"context"
	"fmt"
	"os"

	"github.com/charmbracelet/log"
	"github.com/getsentry/sentry-go"
	openai "github.com/sashabaranov/go-openai"
)

func getOpenAIToken() string {
	token := os.Getenv("OPENAI_APITOKEN")
	if token == "" {
		token = os.Getenv("OPENAI_API_KEY")
	}
	return token
}

func GetAiResponse(labels []string) string {
	client := openai.NewClient(getOpenAIToken())
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Temperature: 1.2,
			Model:       openai.GPT3Dot5Turbo,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: fmt.Sprintf("You are funny David Goggins. Write a response to a user after their workout, congratulating them for their effort and enoucraging them to continue working out, address this list of words in your response: %s. Keep it under 100 characters. End the message with emojis matching the words from the list.", labels),
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

func GetAiWhoopResponse(sport string, strain float64, calories float64, hr int, duration float64) string {
	client := openai.NewClient(getOpenAIToken())
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Temperature: 1.2,
			Model:       openai.GPT3Dot5Turbo,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: fmt.Sprintf("You are funny David Goggins. Write a response to a user after their %s workout. Metrics: Strain %.1f, Calories %.0f, Avg HR %d, Duration %.0f mins. Congratulate them on the effort using the metrics. Keep it under 100 characters. End with emojis.", sport, strain, calories, hr, duration),
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

func GetAiWelcomeResponse() string {
	client := openai.NewClient(getOpenAIToken())
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Temperature: 1.2,
			Model:       openai.GPT3Dot5Turbo,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: "You are funny David Goggins. Write a response to a user after their workout, welcoming them back. Keep it under 100 characters.",
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
