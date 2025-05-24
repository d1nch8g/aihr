package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/d1nch8g/aihr/sound" // Replace with your actual module path

	"github.com/hajimehoshi/go-mp3"
)

func main() {
	// First, get MP3 file info to configure audio properly
	mp3Info, err := getMP3Info("audio.mp3")
	if err != nil {
		log.Fatalf("Failed to get MP3 info: %v", err)
	}

	fmt.Printf("MP3 Info - Sample Rate: %d Hz, Length: %d samples\n",
		mp3Info.SampleRate, mp3Info.Length)

	// Configure audio player with MP3's actual sample rate
	config := sound.PlayerConfig{
		SampleRate:      float64(mp3Info.SampleRate), // Use MP3's sample rate
		FramesPerBuffer: 1024,
		InputChannels:   0,
		OutputChannels:  2, // Stereo output
	}

	player := sound.NewPortaudioPlayer(config)

	if err := player.Initialize(); err != nil {
		log.Fatalf("Failed to initialize PortAudio: %v", err)
	}
	defer player.Terminate()

	if err := player.Open(); err != nil {
		log.Fatalf("Failed to open audio stream: %v", err)
	}
	defer player.Close()

	// Load and decode the MP3 file
	audioData, err := loadMP3FileWithInfo("audio.mp3", mp3Info)
	if err != nil {
		log.Fatalf("Failed to load MP3 file: %v", err)
	}

	// Create a channel to stream audio data
	audioChannel := make(chan []byte, 5)

	// Create context for controlling playback
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupts gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nStopping playback...")
		cancel()
	}()

	// Start playback in a goroutine
	playbackDone := make(chan error, 1)
	go func() {
		playbackDone <- player.StartPlayback(ctx, audioChannel)
	}()

	// Stream audio data
	fmt.Println("Starting audio playback...")
	go streamAudioData(ctx, audioChannel, audioData, config.FramesPerBuffer, 2) // 2 channels

	// Wait for playback to complete or error
	select {
	case err := <-playbackDone:
		if err != nil && err != context.Canceled {
			log.Printf("Playback error: %v", err)
		}
	case <-ctx.Done():
		// Context cancelled
	}

	fmt.Println("Playback finished.")
}

type MP3Info struct {
	SampleRate int
	Length     int64
}

func getMP3Info(filename string) (*MP3Info, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	decoder, err := mp3.NewDecoder(file)
	if err != nil {
		return nil, fmt.Errorf("failed to create MP3 decoder: %w", err)
	}

	return &MP3Info{
		SampleRate: decoder.SampleRate(),
		Length:     decoder.Length(),
	}, nil
}

func loadMP3FileWithInfo(filename string, info *MP3Info) ([]byte, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	decoder, err := mp3.NewDecoder(file)
	if err != nil {
		return nil, fmt.Errorf("failed to create MP3 decoder: %w", err)
	}

	// Pre-allocate slice with estimated size (stereo, 16-bit)
	estimatedSize := info.Length * 4 // 2 channels * 2 bytes per sample
	audioData := make([]byte, 0, estimatedSize)

	// Read in larger chunks for efficiency
	buffer := make([]byte, 16384)

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

	fmt.Printf("Loaded %d bytes of audio data\n", len(audioData))
	return audioData, nil
}

func streamAudioData(ctx context.Context, audioChannel chan<- []byte, audioData []byte, framesPerBuffer int, channels int) {
	defer close(audioChannel)

	// Calculate chunk size: frames * bytes_per_sample * channels
	chunkSize := framesPerBuffer * 2 * channels // 16-bit stereo

	fmt.Printf("Streaming audio in chunks of %d bytes\n", chunkSize)

	for i := 0; i < len(audioData); i += chunkSize {
		select {
		case <-ctx.Done():
			return
		default:
		}

		end := i + chunkSize
		if end > len(audioData) {
			// Last chunk - pad with zeros if necessary
			chunk := make([]byte, chunkSize)
			copy(chunk, audioData[i:])

			select {
			case audioChannel <- chunk:
			case <-ctx.Done():
				return
			}
			break
		} else {
			select {
			case audioChannel <- audioData[i:end]:
			case <-ctx.Done():
				return
			}
		}
	}
}
