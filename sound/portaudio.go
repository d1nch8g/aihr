package sound

import (
	"context"
	"encoding/binary"
	"errors"
	"log"

	"github.com/gordonklaus/portaudio"
)

type PortaudioPlayer struct {
	stream      *portaudio.Stream
	audioBuffer []int16
	config      PlayerConfig
}

func NewPortaudioPlayer(config PlayerConfig) *PortaudioPlayer {
	return &PortaudioPlayer{
		config:      config,
		audioBuffer: make([]int16, config.FramesPerBuffer),
	}
}

func GetDefaultConfig() PlayerConfig {
	return PlayerConfig{
		SampleRate:      44100,
		FramesPerBuffer: 1024,
		InputChannels:   0,
		OutputChannels:  1,
	}
}

func (p *PortaudioPlayer) Initialize() error {
	return portaudio.Initialize()
}

func (p *PortaudioPlayer) Open() error {
	stream, err := portaudio.OpenDefaultStream(
		p.config.InputChannels,
		p.config.OutputChannels,
		p.config.SampleRate,
		p.config.FramesPerBuffer,
		p.audioBuffer,
	)
	if err != nil {
		return err
	}
	p.stream = stream
	return nil
}

func (p *PortaudioPlayer) StartPlayback(ctx context.Context, audioData <-chan []byte) error {
	if p.stream == nil {
		return errors.New("Stream not opened")
	}

	if err := p.stream.Start(); err != nil {
		return err
	}
	defer p.stream.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case audioBytes, ok := <-audioData:
			if !ok {
				return nil // Channel closed
			}

			// Convert bytes to int16 samples
			samples := p.convertBytesToSamples(audioBytes)

			// Copy samples to buffer (pad or truncate as needed)
			bufferLen := len(p.audioBuffer)
			samplesLen := len(samples)

			if samplesLen >= bufferLen {
				copy(p.audioBuffer, samples[:bufferLen])
			} else {
				copy(p.audioBuffer, samples)
				// Zero-fill remaining buffer
				for i := samplesLen; i < bufferLen; i++ {
					p.audioBuffer[i] = 0
				}
			}

			if err := p.stream.Write(); err != nil {
				log.Printf("Error writing audio: %v", err)
				continue
			}

		default:
			// Non-blocking check if no audio data available
		}
	}
}

func (p *PortaudioPlayer) convertBytesToSamples(audioBytes []byte) []int16 {
	samples := make([]int16, len(audioBytes)/2)
	for i := 0; i < len(samples); i++ {
		// Convert little-endian bytes to int16
		samples[i] = int16(binary.LittleEndian.Uint16(audioBytes[i*2 : i*2+2]))
	}
	return samples
}

func (p *PortaudioPlayer) Close() error {
	if p.stream != nil {
		return p.stream.Close()
	}
	return nil
}

func (p *PortaudioPlayer) Terminate() {
	portaudio.Terminate()
}
