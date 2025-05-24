package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/d1nch8g/aihr/stt"
	"github.com/gordonklaus/portaudio"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: go run main_stt.go <iam_token> <folder_id>")
		return
	}

	iamToken := os.Args[1]
	folderID := os.Args[2]

	fmt.Println("Starting speech recognition. Press Ctrl-C to stop.")

	// Setup signal handling
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize STT client
	sttClient, err := stt.NewSTTClient(iamToken, folderID)
	if err != nil {
		log.Fatalf("Failed to create STT client: %v", err)
	}
	defer sttClient.Close()

	// Initialize PortAudio
	portaudio.Initialize()
	defer portaudio.Terminate()

	// Setup audio stream
	const sampleRate = 44100
	const framesPerBuffer = 1024
	audioBuffer := make([]int32, framesPerBuffer)

	stream, err := portaudio.OpenDefaultStream(1, 0, sampleRate, framesPerBuffer, audioBuffer)
	if err != nil {
		log.Fatalf("Failed to open audio stream: %v", err)
	}
	defer stream.Close()

	// Create channels for communication
	audioData := make(chan []byte, 10)
	results := make(chan string, 10)

	// Start STT recognition
	go func() {
		if err := sttClient.StreamRecognize(ctx, audioData, results); err != nil {
			log.Printf("STT error: %v", err)
		}
	}()

	// Start audio capture
	go func() {
		defer close(audioData)

		if err := stream.Start(); err != nil {
			log.Printf("Failed to start stream: %v", err)
			return
		}
		defer stream.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			default:
				if err := stream.Read(); err != nil {
					log.Printf("Error reading audio: %v", err)
					continue
				}

				// Convert int32 samples to bytes (16-bit PCM)
				var buf bytes.Buffer
				for _, sample := range audioBuffer {
					// Convert 32-bit to 16-bit
					sample16 := int16(sample >> 16)
					binary.Write(&buf, binary.LittleEndian, sample16)
				}

				select {
				case audioData <- buf.Bytes():
				case <-ctx.Done():
					return
				default:
					// Drop audio if channel is full
				}
			}
		}
	}()

	// Handle results and signals
	for {
		select {
		case <-sig:
			fmt.Println("\nStopping...")
			cancel()
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
