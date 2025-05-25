package sound

import "context"

// Player defines the interface for audio playback
type Player interface {
	// Initialize initializes the audio playback system
	Initialize() error

	// Terminate terminates the audio playback system
	Terminate()

	// PlayStream plays audio data from a channel
	PlayStream(ctx context.Context, audioData <-chan []byte) error
}
