package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/d1nch8g/aihr/audio"
	"github.com/d1nch8g/aihr/config"
	"github.com/d1nch8g/aihr/stt"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if cfg.IamToken == "" || cfg.FolderID == "" {
		fmt.Println("Error: IAM_TOKEN and FOLDER_ID must be set in .env file")
		fmt.Println("Create a .env file with:")
		fmt.Println("IAM_TOKEN=your_iam_token")
		fmt.Println("FOLDER_ID=your_folder_id")
		fmt.Println("LANGUAGE=en-US  # or ru-RU")
		return
	}

	fmt.Printf("Starting speech recognition (Language: %s). Press Ctrl-C to stop.\n", cfg.Audio.Language)

	// Setup signal handling
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize audio streamer
	audioConfig := audio.Config{
		SampleRate:      cfg.Audio.SampleRate,
		FramesPerBuffer: cfg.Audio.FramesPerBuffer,
		InputChannels:   cfg.Audio.InputChannels,
		OutputChannels:  cfg.Audio.OutputChannels,
	}

	audioStreamer := audio.NewAudioStreamer(audioConfig)

	if err := audioStreamer.Initialize(); err != nil {
		log.Fatalf("Failed to initialize PortAudio: %v", err)
	}
	defer audioStreamer.Terminate()

	if err := audioStreamer.Open(); err != nil {
		log.Fatalf("Failed to open audio stream: %v", err)
	}
	defer audioStreamer.Close()

	// Initialize STT client
	sttConfig := stt.Config{
		IamToken:   cfg.IamToken,
		FolderID:   cfg.FolderID,
		Language:   cfg.Audio.Language,
		SampleRate: int32(cfg.Audio.SampleRate),
	}

	sttClient, err := stt.NewSTTClient(sttConfig)
	if err != nil {
		log.Fatalf("Failed to create STT client: %v", err)
	}
	defer sttClient.Close()

	// Create channels for communication
	audioData := make(chan []byte, 10)
	results := make(chan string, 10)

	// Start STT recognition
	go func() {
		if err := sttClient.StreamRecognize(ctx, audioData, results, int64(cfg.Audio.SampleRate)); err != nil {
			log.Printf("STT error: %v", err)
		}
	}()

	// Start audio capture
	go func() {
		defer close(audioData)
		if err := audioStreamer.StartCapture(ctx, audioData); err != nil && err != context.Canceled {
			log.Printf("Audio capture error: %v", err)
		}
	}()

	// Handle results and signals
	for {
		select {
		case <-sig:
			fmt.Println("\nStopping...")
			cancel()
			// Give some time for graceful shutdown
			time.Sleep(500 * time.Millisecond)
			return
		case result, ok := <-results:
			if !ok {
				return
			}
			fmt.Printf("Recognized: %s\n", result)
		case <-time.After(100 * time.Millisecond):
			// Keep the main loop alive
		}
	}
}
