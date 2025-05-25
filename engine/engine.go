package engine

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/d1nch8g/aihr/audio"
	"github.com/d1nch8g/aihr/gpt"
	"github.com/d1nch8g/aihr/sound"
	"github.com/d1nch8g/aihr/stt"
	"github.com/d1nch8g/aihr/tts"
)

// ConversationEntry represents a single exchange in the conversation
type ConversationEntry struct {
	UserInput  string
	AIResponse string
	Timestamp  time.Time
}

// EngineConfig holds the configuration for the AI-HR engine
type EngineConfig struct {
	SystemPrompt   string
	MaxHistorySize int
	SampleRate     int64
	SilenceTimeout time.Duration
}

// Engine orchestrates the AI-HR conversation flow
type Engine struct {
	config        EngineConfig
	audioStreamer audio.AudioStreamer
	sttClient     stt.STTClient
	gptClient     gpt.GPTClient
	ttsClient     tts.Synthesizer
	soundPlayer   sound.Player

	history      []ConversationEntry
	historyMutex sync.RWMutex

	isRunning    bool
	runningMutex sync.RWMutex
}

// NewEngine creates a new AI-HR engine instance
func NewEngine(
	config EngineConfig,
	audioStreamer audio.AudioStreamer,
	sttClient stt.STTClient,
	gptClient gpt.GPTClient,
	ttsClient tts.Synthesizer,
	soundPlayer sound.Player,
) *Engine {
	if config.MaxHistorySize == 0 {
		config.MaxHistorySize = 10 // Default to last 10 exchanges
	}
	if config.SilenceTimeout == 0 {
		config.SilenceTimeout = 3 * time.Second // Default 3 seconds
	}
	if config.SampleRate == 0 {
		config.SampleRate = 44100 // Default sample rate
	}

	return &Engine{
		config:        config,
		audioStreamer: audioStreamer,
		sttClient:     sttClient,
		gptClient:     gptClient,
		ttsClient:     ttsClient,
		soundPlayer:   soundPlayer,
		history:       make([]ConversationEntry, 0),
	}
}

// Start begins the conversation engine
func (e *Engine) Start(ctx context.Context) error {
	e.runningMutex.Lock()
	if e.isRunning {
		e.runningMutex.Unlock()
		return fmt.Errorf("engine is already running")
	}
	e.isRunning = true
	e.runningMutex.Unlock()

	defer func() {
		e.runningMutex.Lock()
		e.isRunning = false
		e.runningMutex.Unlock()
	}()

	// Initialize audio system
	if err := e.audioStreamer.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize audio streamer: %w", err)
	}
	defer e.audioStreamer.Terminate()

	if err := e.audioStreamer.Open(); err != nil {
		return fmt.Errorf("failed to open audio stream: %w", err)
	}
	defer e.audioStreamer.Close()

	// Initialize sound player
	if err := e.soundPlayer.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize sound player: %w", err)
	}
	defer e.soundPlayer.Terminate()

	log.Println("AI-HR Engine started. Listening for user input...")

	for {
		select {
		case <-ctx.Done():
			log.Println("Engine stopping due to context cancellation")
			return ctx.Err()
		default:
			if err := e.processConversationCycle(ctx); err != nil {
				log.Printf("Error in conversation cycle: %v", err)
				// Continue running unless it's a context cancellation
				if ctx.Err() != nil {
					return ctx.Err()
				}
			}
		}
	}
}

// processConversationCycle handles one complete conversation cycle
func (e *Engine) processConversationCycle(ctx context.Context) error {
	// Capture user audio input
	userInput, err := e.captureUserInput(ctx)
	if err != nil {
		return fmt.Errorf("failed to capture user input: %w", err)
	}

	if strings.TrimSpace(userInput) == "" {
		return nil // Skip empty input
	}

	log.Printf("User said: %s", userInput)

	// Generate AI response
	aiResponse, err := e.generateResponse(userInput)
	if err != nil {
		return fmt.Errorf("failed to generate AI response: %w", err)
	}

	log.Printf("AI response: %s", aiResponse)

	// Convert response to speech and play it
	if err := e.speakResponse(ctx, aiResponse); err != nil {
		return fmt.Errorf("failed to speak response: %w", err)
	}

	// Add to conversation history
	e.addToHistory(ConversationEntry{
		UserInput:  userInput,
		AIResponse: aiResponse,
		Timestamp:  time.Now(),
	})

	return nil
}

// captureUserInput captures and transcribes user audio input
func (e *Engine) captureUserInput(ctx context.Context) (string, error) {
	audioData := make(chan []byte, 100)
	sttResults := make(chan string, 10)

	// Start audio capture
	captureCtx, captureCancel := context.WithCancel(ctx)
	defer captureCancel()

	go func() {
		if err := e.audioStreamer.StartCapture(captureCtx, audioData); err != nil {
			log.Printf("Audio capture error: %v", err)
		}
		close(audioData)
	}()

	// Start STT processing
	sttCtx, sttCancel := context.WithCancel(ctx)
	defer sttCancel()

	go func() {
		if err := e.sttClient.StreamRecognize(sttCtx, audioData, sttResults, e.config.SampleRate); err != nil {
			log.Printf("STT error: %v", err)
		}
		close(sttResults)
	}()

	// Collect STT results with silence timeout
	var transcription strings.Builder
	silenceTimer := time.NewTimer(e.config.SilenceTimeout)
	defer silenceTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case result, ok := <-sttResults:
			if !ok {
				return transcription.String(), nil
			}
			if result != "" {
				transcription.WriteString(result)
				transcription.WriteString(" ")
				// Reset silence timer on new input
				if !silenceTimer.Stop() {
					<-silenceTimer.C
				}
				silenceTimer.Reset(e.config.SilenceTimeout)
			}
		case <-silenceTimer.C:
			// Silence timeout reached, stop capturing
			captureCancel()
			sttCancel()
			return transcription.String(), nil
		}
	}
}

// generateResponse creates an AI response using the GPT client
func (e *Engine) generateResponse(userInput string) (string, error) {
	systemMessage := e.buildSystemMessage()
	return e.gptClient.Complete(systemMessage, userInput)
}

// speakResponse converts text to speech and plays it
func (e *Engine) speakResponse(ctx context.Context, text string) error {
	audioData := make(chan []byte, 100)

	// Start TTS synthesis
	ttsCtx, ttsCancel := context.WithCancel(ctx)
	defer ttsCancel()

	synthesisOptions := tts.SynthesisOptions{
		Voice:  "jane", // Default voice
		Speed:  1.0,
		Volume: 1.0,
		Model:  "tts-1", // Default model
	}

	go func() {
		if err := e.ttsClient.SynthesizeToStreamWithContext(ttsCtx, text, synthesisOptions, audioData); err != nil {
			log.Printf("TTS synthesis error: %v", err)
		}
		close(audioData)
	}()

	// Play the audio
	return e.soundPlayer.PlayStream(ctx, audioData)
}

// buildSystemMessage constructs the system message with conversation history
func (e *Engine) buildSystemMessage() string {
	e.historyMutex.RLock()
	defer e.historyMutex.RUnlock()

	var systemMessage strings.Builder

	// Add conversation history
	if len(e.history) > 0 {
		systemMessage.WriteString("Previous conversation history:\n")
		for _, entry := range e.history {
			systemMessage.WriteString(fmt.Sprintf("User: %s\n", entry.UserInput))
			systemMessage.WriteString(fmt.Sprintf("Assistant: %s\n", entry.AIResponse))
			systemMessage.WriteString("---\n")
		}
		systemMessage.WriteString("\n")
	}

	// Add the main system prompt
	systemMessage.WriteString(e.config.SystemPrompt)

	return systemMessage.String()
}

// addToHistory adds a conversation entry to the history
func (e *Engine) addToHistory(entry ConversationEntry) {
	e.historyMutex.Lock()
	defer e.historyMutex.Unlock()

	e.history = append(e.history, entry)

	// Trim history if it exceeds max size
	if len(e.history) > e.config.MaxHistorySize {
		e.history = e.history[len(e.history)-e.config.MaxHistorySize:]
	}
}

// GetHistory returns a copy of the conversation history
func (e *Engine) GetHistory() []ConversationEntry {
	e.historyMutex.RLock()
	defer e.historyMutex.RUnlock()

	history := make([]ConversationEntry, len(e.history))
	copy(history, e.history)
	return history
}

// ClearHistory clears the conversation history
func (e *Engine) ClearHistory() {
	e.historyMutex.Lock()
	defer e.historyMutex.Unlock()

	e.history = e.history[:0]
}

// IsRunning returns whether the engine is currently running
func (e *Engine) IsRunning() bool {
	e.runningMutex.RLock()
	defer e.runningMutex.RUnlock()
	return e.isRunning
}

// Stop gracefully stops the engine
func (e *Engine) Stop() error {
	// Close all clients
	var errors []error

	if err := e.sttClient.Close(); err != nil {
		errors = append(errors, fmt.Errorf("failed to close STT client: %w", err))
	}

	if err := e.ttsClient.Close(); err != nil {
		errors = append(errors, fmt.Errorf("failed to close TTS client: %w", err))
	}

	if len(errors) > 0 {
		var errorStrings []string
		for _, err := range errors {
			errorStrings = append(errorStrings, err.Error())
		}
		return fmt.Errorf("errors during shutdown: %s", strings.Join(errorStrings, "; "))
	}

	return nil
}
