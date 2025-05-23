package main

import (
	"fmt"
	"log"

	"github.com/d1nch8g/aihr/config"
	"github.com/d1nch8g/aihr/gpt"
)

type Client struct {
	FolderID string
	IAMToken string
}

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Create a client with credentials
	client := gpt.NewClient(cfg.GPTFolderID, cfg.IamToken)

	// Create the request
	req := gpt.Request{
		ModelURI: "gpt://b1g5ju6e4ke0kp5eqg27/yandexgpt/rc",
		CompletionOptions: gpt.CompletionOptions{
			MaxTokens:   500,
			Temperature: 0.3,
		},
		Messages: []gpt.Message{
			{
				Role: "system",
				Text: "Ты помогаешь решить вопрос с собеседования по программированию на языке Go.",
			},
			{
				Role: "user",
				Text: "Дано: два неупорядоченных среза.\nа) a := []int{37, 5, 1, 2} и b := []int{6, 2, 4, 37}.\nб) a = []int{1, 1, 1} и b = []int{1, 1, 1, 1}.\nВерните их пересечение.",
			},
		},
	}

	// Send the request
	resp, err := client.Complete(req)
	if err != nil {
		log.Fatalf("Failed to complete request: %v", err)
	}

	// Print the response
	if len(resp.Result.Alternatives) > 0 {
		fmt.Println("Response:")
		fmt.Println(resp.Result.Alternatives[0].Message.Text)
	} else {
		fmt.Println("No response alternatives received")
	}

	// Print usage statistics
	fmt.Printf("Usage: Input tokens: %d, Output tokens: %d, Total tokens: %d\n",
		resp.Result.Usage.InputTextTokens,
		resp.Result.Usage.OutputTextTokens,
		resp.Result.Usage.TotalTokens)

}
