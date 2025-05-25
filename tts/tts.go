package tts

import "context"

// Synthesizer defines the interface for text-to-speech synthesis
type Synthesizer interface {
	SynthesizeToStreamWithContext(ctx context.Context, text string, options SynthesisOptions, audioData chan<- []byte) error
	Close() error
}

// SynthesisOptions represents the configuration for speech synthesis
type SynthesisOptions struct {
	Voice                 string
	Speed                 float64
	Volume                float64
	Model                 string
	Format                interface{} // Will be specific to implementation
	LoudnessNormalization interface{} // Will be specific to implementation
}
