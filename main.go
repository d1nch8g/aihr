// Unified AI-HR interview system with STT, GPT, and TTS
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/d1nch8g/aihr/audio"
	"github.com/d1nch8g/aihr/config"
	"github.com/d1nch8g/aihr/gpt"
	"github.com/d1nch8g/aihr/sound"
	"github.com/d1nch8g/aihr/stt"
	"github.com/d1nch8g/aihr/tts"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	fmt.Printf("Starting AI-HR interview system (Language: %s). Press Ctrl-C to stop.\n", cfg.Audio.Language)

	// Setup signal handling
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize audio streamer for recording
	audioConfig := audio.PortaudioConfig{
		SampleRate:      cfg.Audio.SampleRate,
		FramesPerBuffer: cfg.Audio.FramesPerBuffer,
		InputChannels:   cfg.Audio.InputChannels,
		OutputChannels:  cfg.Audio.OutputChannels,
	}

	audioStreamer := audio.NewPortaudioStreamer(audioConfig)
	if err := audioStreamer.Initialize(); err != nil {
		log.Fatalf("Failed to initialize PortAudio for recording: %v", err)
	}
	defer audioStreamer.Terminate()

	if err := audioStreamer.Open(); err != nil {
		log.Fatalf("Failed to open audio stream for recording: %v", err)
	}
	defer audioStreamer.Close()

	// Initialize audio player for TTS playback
	playerConfig := sound.PlayerConfig{
		SampleRate:      22050.0,
		FramesPerBuffer: 2048,
		InputChannels:   0,
		OutputChannels:  1,
	}

	player := sound.NewPortaudioPlayer(playerConfig)
	if err := player.Initialize(); err != nil {
		log.Fatalf("Failed to initialize PortAudio for playback: %v", err)
	}
	defer player.Terminate()

	if err := player.Open(); err != nil {
		log.Fatalf("Failed to open audio stream for playback: %v", err)
	}
	defer player.Close()

	// Initialize STT client
	sttConfig := stt.YandexConfig{
		IamToken:   cfg.IamToken,
		FolderID:   cfg.FolderID,
		Language:   cfg.Audio.Language,
		SampleRate: int32(cfg.Audio.SampleRate),
	}

	sttClient, err := stt.NewYandexSTTClient(sttConfig)
	if err != nil {
		log.Fatalf("Failed to create STT client: %v", err)
	}
	defer sttClient.Close()

	// Initialize TTS client
	ttsConfig := tts.YandexConfig{
		IamToken: cfg.IamToken,
		FolderID: cfg.FolderID,
	}

	ttsClient, err := tts.NewYandexTTSClient(ttsConfig)
	if err != nil {
		log.Fatalf("Failed to create TTS client: %v", err)
	}
	defer ttsClient.Close()

	// Initialize GPT client
	gptClient := gpt.NewYandexGPTClient(cfg.FolderID, cfg.IamToken)

	// Create channels for communication
	audioData := make(chan []byte, 10)
	sttResults := make(chan string, 10)
	gptResponses := make(chan string, 10)

	// Start STT recognition
	go func() {
		if err := sttClient.StreamRecognize(ctx, audioData, sttResults, int64(cfg.Audio.SampleRate)); err != nil {
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

	// Process STT results with GPT
	go func() {
		defer close(gptResponses)
		for {
			select {
			case <-ctx.Done():
				return
			case result, ok := <-sttResults:
				if !ok {
					return
				}

				fmt.Printf("User: %s\n", result)

				reply, err := gptClient.Complete("Ты HR проводящий собеседование на go разработчика", result)
				if err != nil {
					log.Printf("GPT error: %v", err)
					continue
				}

				fmt.Printf("GPT: %s\n", reply)

				select {
				case gptResponses <- reply:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	// Process GPT responses with TTS and play them
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case response, ok := <-gptResponses:
				if !ok {
					return
				}

				// Play the GPT response using TTS
				if err := playTTSResponse(ctx, ttsClient, player, response, playerConfig); err != nil {
					log.Printf("TTS playback error: %v", err)
				}
			}
		}
	}()

	// Play welcome message
	welcomeMsg := "Hello! Welcome to the AI-HR interview system. I will be conducting your interview today. Please introduce yourself and tell me about your experience with Go development."
	fmt.Printf("AI-HR: %s\n", welcomeMsg)
	if err := playTTSResponse(ctx, ttsClient, player, welcomeMsg, playerConfig); err != nil {
		log.Printf("Welcome message TTS error: %v", err)
	}

	// Main loop - handle signals
	for {
		select {
		case <-sig:
			fmt.Println("\nStopping AI-HR interview system...")
			cancel()
			// Give some time for graceful shutdown
			time.Sleep(1 * time.Second)
			return
		case <-ctx.Done():
			return
		case <-time.After(100 * time.Millisecond):
			// Keep the main loop alive
		}
	}
}

// playTTSResponse synthesizes text to speech and plays it back
func playTTSResponse(ctx context.Context, ttsClient *tts.YandexTTSClient, player *sound.PortaudioPlayer, text string, playerConfig sound.PlayerConfig) error {
	// Get default synthesis options
	options := tts.GetDefaultSynthesisOptions()
	options.Voice = "marina"
	options.Speed = 1.0
	options.Volume = 0.0

	// Create context with timeout for TTS
	ttsCtx, ttsCancel := context.WithTimeout(ctx, 30*time.Second)
	defer ttsCancel()

	// Create context for playback control
	playCtx, playCancel := context.WithCancel(ctx)
	defer playCancel()

	// Create channels for audio data flow
	ttsAudioData := make(chan []byte, 100)
	playbackAudioData := make(chan []byte, 10)

	// Start TTS synthesis
	synthesisComplete := make(chan error, 1)
	go func() {
		synthesisComplete <- ttsClient.SynthesizeToStreamWithContext(ttsCtx, text, options, ttsAudioData)
	}()

	// Start audio playback
	playbackComplete := make(chan error, 1)
	go func() {
		playbackComplete <- player.PlayStream(playCtx, playbackAudioData)
	}()

	// Process and stream audio data from TTS to playback
	go func() {
		defer close(playbackAudioData)

		var audioBuffer []byte
		chunkSize := playerConfig.FramesPerBuffer * 2 * playerConfig.OutputChannels

		for {
			select {
			case chunk, ok := <-ttsAudioData:
				if !ok {
					// TTS finished, flush remaining buffer
					if len(audioBuffer) > 0 {
						if len(audioBuffer) < chunkSize {
							padded := make([]byte, chunkSize)
							copy(padded, audioBuffer)
							audioBuffer = padded
						}

						select {
						case playbackAudioData <- audioBuffer:
						case <-playCtx.Done():
							return
						}
					}
					return
				}

				// Add chunk to buffer
				audioBuffer = append(audioBuffer, chunk...)

				// Send complete chunks to playback
				for len(audioBuffer) >= chunkSize {
					select {
					case playbackAudioData <- audioBuffer[:chunkSize]:
						audioBuffer = audioBuffer[chunkSize:]
					case <-playCtx.Done():
						return
					}
				}

			case <-playCtx.Done():
				return
			}
		}
	}()

	// Wait for synthesis to complete
	select {
	case err := <-synthesisComplete:
		if err != nil && err != context.Canceled {
			return fmt.Errorf("synthesis error: %v", err)
		}
	case <-ttsCtx.Done():
		return fmt.Errorf("synthesis timed out or cancelled")
	}

	// Wait for playback to complete
	select {
	case err := <-playbackComplete:
		if err != nil && err != context.Canceled {
			return fmt.Errorf("playback error: %v", err)
		}
	case <-playCtx.Done():
		// Context cancelled
	}

	return nil
}
