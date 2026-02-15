package instagram

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/charmbracelet/log"
	"github.com/spf13/viper"
)

type ContainerResponse struct {
	ID string `json:"id"`
}

type PublishResponse struct {
	ID string `json:"id"`
}

func PublishStory(imageURL, caption string) (string, error) {
	log.Debug("Publishing story to Instagram", "imageURL", imageURL)
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
	if caption != "" {
		params.Set("caption", caption)
	}
	params.Set("access_token", accessToken)

	log.Debug("Creating story container")
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
	log.Debug("Story container created", "containerID", containerRes.ID)

	// 2. Wait for container to be ready
	if err := waitForContainer(containerRes.ID, accessToken); err != nil {
		return "", err
	}

	// 3. Publish Media Container
	publishURL := fmt.Sprintf("https://graph.facebook.com/v18.0/%s/media_publish", businessID)
	params = url.Values{}
	params.Set("creation_id", containerRes.ID)
	params.Set("access_token", accessToken)

	log.Debug("Publishing story container")
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
	log.Debug("Story published successfully", "storyID", publishRes.ID)

	return publishRes.ID, nil
}

func PublishPost(imageURL, caption string) (string, error) {
	log.Debug("Publishing post to Instagram", "imageURL", imageURL)
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

	log.Debug("Creating post container")
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
	log.Debug("Post container created", "containerID", containerRes.ID)

	// 2. Wait for container to be ready
	if err := waitForContainer(containerRes.ID, accessToken); err != nil {
		return "", err
	}

	// 3. Publish Media Container
	publishURL := fmt.Sprintf("https://graph.facebook.com/v18.0/%s/media_publish", businessID)
	params = url.Values{}
	params.Set("creation_id", containerRes.ID)
	params.Set("access_token", accessToken)

	log.Debug("Publishing post container")
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
	log.Debug("Post published successfully", "postID", publishRes.ID)

	return publishRes.ID, nil
}

func waitForContainer(containerID, accessToken string) error {
	checkURL := fmt.Sprintf("https://graph.facebook.com/v18.0/%s", containerID)
	params := url.Values{}
	params.Set("fields", "status_code")
	params.Set("access_token", accessToken)

	log.Debug("Waiting for media container to be processed by Meta...", "containerID", containerID)

	for i := 0; i < 12; i++ { // Wait up to 60 seconds (12 * 5s)
		resp, err := http.Get(checkURL + "?" + params.Encode())
		if err != nil {
			return err
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		log.Debug("Container status check response", "body", string(body))

		var status struct {
			StatusCode string `json:"status_code"`
		}
		if err := json.Unmarshal(body, &status); err != nil {
			return fmt.Errorf("failed to decode status: %s", string(body))
		}

		if status.StatusCode == "FINISHED" {
			log.Debug("Container processed and ready", "containerID", containerID)
			return nil
		}
		if status.StatusCode == "ERROR" {
			return fmt.Errorf("container processing failed: %s", string(body))
		}

		log.Debug("Container not ready yet, waiting 5s...", "status", status.StatusCode, "containerID", containerID)
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("timed out waiting for container %s to be READY", containerID)
}
