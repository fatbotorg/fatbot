package instagram

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/spf13/viper"
)

type ContainerResponse struct {
	ID string `json:"id"`
}

type PublishResponse struct {
	ID string `json:"id"`
}

func PublishStory(imageURL string) (string, error) {
	businessID := viper.GetString("instagram.business_account_id")
	accessToken := viper.GetString("instagram.access_token")

	if businessID == "" || accessToken == "" {
		return "", fmt.Errorf("instagram credentials not configured")
	}

	// 1. Create Media Container for Story
	containerURL := fmt.Sprintf("https://graph.facebook.com/v18.0/%s/media", businessID)
	params := url.Values{}
	params.Set("image_url", imageURL)
	params.Set("media_type", "STORIES")
	params.Set("access_token", accessToken)

	resp, err := http.PostForm(containerURL, params)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var containerRes ContainerResponse
	if err := json.Unmarshal(body, &containerRes); err != nil {
		return "", fmt.Errorf("failed to decode response: %s", string(body))
	}

	if containerRes.ID == "" {
		return "", fmt.Errorf("failed to create story container: %s", string(body))
	}

	// 2. Wait for container to be ready
	time.Sleep(10 * time.Second)

	// 3. Publish Media Container
	publishURL := fmt.Sprintf("https://graph.facebook.com/v18.0/%s/media_publish", businessID)
	params = url.Values{}
	params.Set("creation_id", containerRes.ID)
	params.Set("access_token", accessToken)

	resp, err = http.PostForm(publishURL, params)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ = io.ReadAll(resp.Body)
	var publishRes PublishResponse
	if err := json.Unmarshal(body, &publishRes); err != nil {
		return "", fmt.Errorf("failed to decode publish response: %s", string(body))
	}

	if publishRes.ID == "" {
		return "", fmt.Errorf("failed to publish: %s", string(body))
	}

	return publishRes.ID, nil
}

func PublishPost(imageURL, caption string) (string, error) {
	businessID := viper.GetString("instagram.business_account_id")
	accessToken := viper.GetString("instagram.access_token")

	if businessID == "" || accessToken == "" {
		return "", fmt.Errorf("instagram credentials not configured")
	}

	// 1. Create Media Container for Post
	containerURL := fmt.Sprintf("https://graph.facebook.com/v18.0/%s/media", businessID)
	params := url.Values{}
	params.Set("image_url", imageURL)
	params.Set("caption", caption)
	params.Set("access_token", accessToken)

	resp, err := http.PostForm(containerURL, params)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var containerRes ContainerResponse
	if err := json.Unmarshal(body, &containerRes); err != nil {
		return "", fmt.Errorf("failed to decode response: %s", string(body))
	}

	if containerRes.ID == "" {
		return "", fmt.Errorf("failed to create post container: %s", string(body))
	}

	// 2. Wait for container to be ready
	time.Sleep(15 * time.Second)

	// 3. Publish Media Container
	publishURL := fmt.Sprintf("https://graph.facebook.com/v18.0/%s/media_publish", businessID)
	params = url.Values{}
	params.Set("creation_id", containerRes.ID)
	params.Set("access_token", accessToken)

	resp, err = http.PostForm(publishURL, params)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ = io.ReadAll(resp.Body)
	var publishRes PublishResponse
	if err := json.Unmarshal(body, &publishRes); err != nil {
		return "", fmt.Errorf("failed to decode publish response: %s", string(body))
	}

	if publishRes.ID == "" {
		return "", fmt.Errorf("failed to publish: %s", string(body))
	}

	return publishRes.ID, nil
}
