package gpt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	YandexGPTEndpoint = "https://llm.api.cloud.yandex.net/foundationModels/v1/completion"
)

// Message represents a message in the conversation
type Message struct {
	Role string `json:"role"`
	Text string `json:"text"`
}

// CompletionOptions represents the options for the completion
type CompletionOptions struct {
	MaxTokens   int     `json:"maxTokens"`
	Temperature float64 `json:"temperature"`
}

// Request represents the request to the Yandex GPT API
type Request struct {
	ModelURI          string            `json:"modelUri"`
	CompletionOptions CompletionOptions `json:"completionOptions"`
	Messages          []Message         `json:"messages"`
}

// Alternative represents an alternative response
type Alternative struct {
	Message Message `json:"message"`
	Status  string  `json:"status"`
}

// Response represents the response from the Yandex GPT API
type Response struct {
	Result struct {
		Alternatives []Alternative `json:"alternatives"`
		Usage        struct {
			InputTextTokens         string `json:"inputTextTokens"`
			CompletionTokens        string `json:"completionTokens"`
			TotalTokens             string `json:"totalTokens"`
			CompletionTokensDetails struct {
				ReasoningTokens string `json:"reasoningTokens"`
			} `json:"completionTokensDetails"`
		} `json:"usage"`
		ModelVersion string `json:"modelVersion"`
	} `json:"result"`
}

// Client is a client for the Yandex GPT API
type Client struct {
	FolderID   string
	IAMToken   string
	HTTPClient *http.Client
}

// NewClient creates a new Yandex GPT client
func NewClient(folderID, iamToken string) *Client {
	return &Client{
		FolderID:   folderID,
		IAMToken:   iamToken,
		HTTPClient: &http.Client{},
	}
}

// Complete sends a completion request to the Yandex GPT API
func (c *Client) Complete(req Request) (*Response, error) {
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", YandexGPTEndpoint, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.IAMToken)
	httpReq.Header.Set("x-folder-id", c.FolderID)

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var response Response
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}
