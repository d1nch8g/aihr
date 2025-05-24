package audio

import "context"

// AudioStreamer defines the interface for audio streaming implementations
type AudioStreamer interface {
	// Initialize initializes the audio system
	Initialize() error

	// Terminate terminates the audio system
	Terminate()

	// Open opens the audio stream with configured parameters
	Open() error

	// Close closes the audio stream
	Close() error

	// StartCapture starts capturing audio and sends data to the provided channel
	// The method blocks until the context is cancelled
	StartCapture(ctx context.Context, audioData chan<- []byte) error
}
