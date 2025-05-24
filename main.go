package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/d1nch8g/aihr/sound"

	"github.com/hajimehoshi/go-mp3"
)

func main() {
	// Initialize the audio player
	config := sound.GetDefaultConfig()
	player := sound.NewPortaudioPlayer(config)

	if err := player.Initialize(); err != nil {
		log.Fatalf("Failed to initialize PortAudio: %v", err)
	}
	defer player.Terminate()

	if err := player.Open(); err != nil {
		log.Fatalf("Failed to open audio stream: %v", err)
	}
	defer player.Close()

	// Open and decode the MP3 file
	audioData, err := loadMP3File("audio.mp3", int(config.SampleRate))
	if err != nil {
		log.Fatalf("Failed to load MP3 file: %v", err)
	}

	// Create a channel to stream audio data
	audioChannel := make(chan []byte, 10)

	// Create context for controlling playback
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start playback in a goroutine
	go func() {
		if err := player.StartPlayback(ctx, audioChannel); err != nil {
			log.Printf("Playback error: %v", err)
		}
	}()

	// Stream audio data
	fmt.Println("Starting audio playback...")
	go streamAudioData(audioChannel, audioData, config.FramesPerBuffer)

	// Wait for playback to complete
	duration := time.Duration(len(audioData)/2/int(config.SampleRate)) * time.Second
	fmt.Printf("Playing audio for approximately %v...\n", duration)

	time.Sleep(duration + 2*time.Second) // Add extra time for buffer

	// Stop playback
	cancel()
	close(audioChannel)

	fmt.Println("Playback finished.")
}

func loadMP3File(filename string, targetSampleRate int) ([]byte, error) {
	// Open the MP3 file
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Create MP3 decoder
	decoder, err := mp3.NewDecoder(file)
	if err != nil {
		return nil, fmt.Errorf("failed to create MP3 decoder: %w", err)
	}

	fmt.Printf("MP3 Info - Sample Rate: %d Hz, Length: %d samples\n",
		decoder.SampleRate(), decoder.Length())

	// Read all audio data
	audioData := make([]byte, 0)
	buffer := make([]byte, 4096)

	for {
		n, err := decoder.Read(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read MP3 data: %w", err)
		}

		audioData = append(audioData, buffer[:n]...)
	}

	// If sample rates don't match, you might need to resample
	// For this example, we'll assume they match or are close enough
	if decoder.SampleRate() != targetSampleRate {
		fmt.Printf("Warning: MP3 sample rate (%d) doesn't match target (%d)\n",
			decoder.SampleRate(), targetSampleRate)
	}

	return audioData, nil
}

func streamAudioData(audioChannel chan<- []byte, audioData []byte, framesPerBuffer int) {
	// Calculate chunk size (frames * 2 bytes per sample * channels)
	chunkSize := framesPerBuffer * 2 // 16-bit mono

	for i := 0; i < len(audioData); i += chunkSize {
		end := i + chunkSize
		if end > len(audioData) {
			end = len(audioData)
			// Pad the last chunk if necessary
			chunk := make([]byte, chunkSize)
			copy(chunk, audioData[i:end])
			audioChannel <- chunk
		} else {
			audioChannel <- audioData[i:end]
		}

		// Add small delay to simulate real-time streaming
		time.Sleep(time.Duration(framesPerBuffer*1000/44100) * time.Millisecond)
	}
}
