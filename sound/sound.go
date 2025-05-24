package sound

import "context"

// Player defines the interface for audio playback
type Player interface {
	Initialize() error
	Open() error
	StartPlayback(ctx context.Context, audioData <-chan []byte) error
	Close() error
	Terminate()
}

// PlayerConfig represents the configuration for audio playback
type PlayerConfig struct {
	SampleRate      float64
	FramesPerBuffer int
	InputChannels   int
	OutputChannels  int
}
