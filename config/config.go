package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	IamToken    string
	GPTFolderID string
}

func LoadConfig() (*Config, error) {
	err := godotenv.Load()
	if err != nil {
		return nil, err
	}

	return &Config{
		IamToken:    os.Getenv("IAM_TOKEN"),
		GPTFolderID: os.Getenv("GPT_FOLDER_ID"),
	}, nil
}
