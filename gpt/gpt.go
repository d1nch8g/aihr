package gpt

// GPTClient defines the interface for GPT API clients
type GPTClient interface {
	// Complete sends a completion request and returns the response
	Complete(systemMessage, userMessage string) (string, error)
}
