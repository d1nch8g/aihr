package stt

import "context"

// STTClient defines the interface for speech-to-text implementations
type STTClient interface {
	// StreamRecognize performs streaming speech recognition
	// audioData: channel receiving audio chunks
	// results: channel for sending recognized text
	// sampleRate: audio sample rate in Hz
	StreamRecognize(ctx context.Context, audioData <-chan []byte, results chan<- string, sampleRate int64) error

	// Close closes the STT client and cleans up resources
	Close() error
}
