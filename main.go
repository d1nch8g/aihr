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
	"github.com/d1nch8g/aihr/engine"
	"github.com/d1nch8g/aihr/gpt"
	"github.com/d1nch8g/aihr/sound"
	"github.com/d1nch8g/aihr/stt"
	"github.com/d1nch8g/aihr/tts"
)

const (
	defaultSystemPrompt = `You are an experienced HR professional conducting a technical interview for a Go developer position. 
Your role is to:
- Ask relevant technical questions about Go programming
- Evaluate the candidate's experience and skills
- Provide constructive feedback
- Keep the conversation professional and engaging
- Ask follow-up questions based on the candidate's responses

Please keep your responses concise and conversational. Start by asking the candidate to introduce themselves.`
)

func main() {
	fmt.Println("Starting AI-HR Interview System...")

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Setup signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nShutting down AI-HR system...")
		cancel()
	}()

	// Initialize all components
	audioStreamer, sttClient, gptClient, ttsClient, soundPlayer, err := initializeComponents(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize components: %v", err)
	}

	// Create engine configuration
	engineConfig := engine.EngineConfig{
		SystemPrompt:   defaultSystemPrompt,
		MaxHistorySize: 10,
		SampleRate:     int64(cfg.Audio.SampleRate),
		SilenceTimeout: 3 * time.Second,
	}

	// Create the AI-HR engine
	aiEngine := engine.NewEngine(
		engineConfig,
		audioStreamer,
		sttClient,
		gptClient,
		ttsClient,
		soundPlayer,
	)

	// Start the interview with a welcome message
	if err := playWelcomeMessage(ctx, ttsClient, soundPlayer); err != nil {
		log.Printf("Failed to play welcome message: %v", err)
	}

	fmt.Printf("AI-HR Interview System ready (Language: %s)\n", cfg.Audio.Language)
	fmt.Println("The AI interviewer will now start the conversation...")
	fmt.Println("Press Ctrl+C to stop the interview at any time.")

	// Start the engine
	if err := aiEngine.Start(ctx); err != nil && err != context.Canceled {
		log.Printf("Engine error: %v", err)
	}

	// Graceful shutdown
	fmt.Println("Stopping AI-HR engine...")
	if err := aiEngine.Stop(); err != nil {
		log.Printf("Error during engine shutdown: %v", err)
	}

	// Display conversation summary
	displayConversationSummary(aiEngine)

	fmt.Println("AI-HR Interview System stopped. Thank you!")
}

// initializeComponents initializes all the required components for the AI-HR system
func initializeComponents(cfg *config.Config) (
	audio.AudioStreamer,
	stt.STTClient,
	gpt.GPTClient,
	tts.Synthesizer,
	sound.Player,
	error,
) {
	// Initialize audio streamer for input
	audioConfig := audio.PortaudioConfig{
		SampleRate:      cfg.Audio.SampleRate,
		FramesPerBuffer: cfg.Audio.FramesPerBuffer,
		InputChannels:   cfg.Audio.InputChannels,
		OutputChannels:  0, // Input only
	}
	audioStreamer := audio.NewPortaudioStreamer(audioConfig)

	// Initialize STT client
	sttConfig := stt.YandexConfig{
		IamToken:   cfg.IamToken,
		FolderID:   cfg.FolderID,
		Language:   cfg.Audio.Language,
		SampleRate: int32(cfg.Audio.SampleRate),
	}
	sttClient, err := stt.NewYandexSTTClient(sttConfig)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("failed to create STT client: %w", err)
	}

	// Initialize GPT client
	gptClient := gpt.NewYandexGPTClient(cfg.FolderID, cfg.IamToken)

	// Initialize TTS client
	ttsConfig := tts.YandexConfig{
		ApiKey:   cfg.IamToken, // Using IAM token as API key for Yandex
		FolderID: cfg.FolderID,
	}
	ttsClient, err := tts.NewYandexTTSClient(ttsConfig)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("failed to create TTS client: %w", err)
	}

	// Initialize sound player for output
	playerConfig := sound.PlayerConfig{
		SampleRate:      22050.0, // Yandex TTS output rate
		FramesPerBuffer: 2048,
		InputChannels:   0,
		OutputChannels:  1, // Mono output
	}
	soundPlayer := sound.NewPortaudioPlayer(playerConfig)

	return audioStreamer, sttClient, gptClient, ttsClient, soundPlayer, nil
}

// playWelcomeMessage plays an initial welcome message to start the interview
func playWelcomeMessage(ctx context.Context, ttsClient tts.Synthesizer, soundPlayer sound.Player) error {
	welcomeText := "Hello! Welcome to the AI-HR interview system. I will be conducting your technical interview for the Go developer position today. Please introduce yourself and tell me about your programming experience."

	// Initialize sound player
	if err := soundPlayer.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize sound player: %w", err)
	}
	defer soundPlayer.Terminate()

	if err := soundPlayer.Initialize(); err != nil {
		return fmt.Errorf("failed to open sound player: %w", err)
	}
	defer soundPlayer.Terminate()

	// Create channels for audio streaming
	ttsAudioData := make(chan []byte, 100)
	playbackAudioData := make(chan []byte, 10)

	// TTS synthesis options
	options := tts.GetDefaultSynthesisOptions()
	options.Voice = "marina"
	options.Speed = 1.0
	options.Volume = 0.0

	// Create contexts
	ttsCtx, ttsCancel := context.WithTimeout(ctx, 30*time.Second)
	defer ttsCancel()

	playCtx, playCancel := context.WithCancel(ctx)
	defer playCancel()

	// Start TTS synthesis
	synthesisComplete := make(chan error, 1)
	go func() {
		synthesisComplete <- ttsClient.SynthesizeToStreamWithContext(ttsCtx, welcomeText, options, ttsAudioData)
	}()

	// Start playback
	playbackComplete := make(chan error, 1)
	go func() {
		playbackComplete <- soundPlayer.PlayStream(playCtx, playbackAudioData)
	}()

	// Stream audio from TTS to playback
	go func() {
		defer close(playbackAudioData)

		chunkSize := 2048 * 2 // 16-bit mono samples
		var audioBuffer []byte

		for {
			select {
			case chunk, ok := <-ttsAudioData:
				if !ok {
					// Flush remaining buffer
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

				audioBuffer = append(audioBuffer, chunk...)

				// Send complete chunks
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
			return fmt.Errorf("synthesis error: %w", err)
		}
	case <-ttsCtx.Done():
		return fmt.Errorf("synthesis timed out")
	}

	// Wait for playback to complete
	select {
	case err := <-playbackComplete:
		if err != nil && err != context.Canceled {
			return fmt.Errorf("playback error: %w", err)
		}
	case <-playCtx.Done():
		// Context cancelled
	}

	fmt.Println("Welcome message played successfully!")
	return nil
}

// displayConversationSummary shows a summary of the interview conversation
func displayConversationSummary(aiEngine *engine.Engine) {
	history := aiEngine.GetHistory()

	if len(history) == 0 {
		fmt.Println("No conversation history to display.")
		return
	}

	fmt.Println("\n" + "====")
	fmt.Println("INTERVIEW CONVERSATION SUMMARY")
	fmt.Println("====")

	for i, entry := range history {
		fmt.Printf("\n--- Exchange %d [%s] ---\n", i+1, entry.Timestamp.Format("15:04:05"))
		fmt.Printf("Candidate: %s\n", entry.UserInput)
		fmt.Printf("Interviewer: %s\n", entry.AIResponse)
	}

	fmt.Println("\n" + "====")
	fmt.Printf("Total exchanges: %d\n", len(history))
	fmt.Printf("Interview duration: %s\n",
		history[len(history)-1].Timestamp.Sub(history[0].Timestamp).Round(time.Second))
	fmt.Println("====")
}
