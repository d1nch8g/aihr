package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	IamToken string
	FolderID string
	Audio    AudioConfig
}

type AudioConfig struct {
	SampleRate      float64
	FramesPerBuffer int
	InputChannels   int
	OutputChannels  int
	Language        string
}

func LoadConfig() (*Config, error) {
	err := godotenv.Load()
	if err != nil {
		return nil, err
	}

	// Set default audio config
	audioConfig := AudioConfig{
		SampleRate:      44100,
		FramesPerBuffer: 1024,
		InputChannels:   1,
		OutputChannels:  0,
		Language:        getEnvOrDefault("LANGUAGE", "en-US"),
	}

	return &Config{
		IamToken: os.Getenv("IAM_TOKEN"),
		FolderID: os.Getenv("FOLDER_ID"),
		Audio:    audioConfig,
	}, nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
