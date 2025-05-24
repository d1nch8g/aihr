package audio

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"log"

	"github.com/gordonklaus/portaudio"
)

type Config struct {
	SampleRate      float64
	FramesPerBuffer int
	InputChannels   int
	OutputChannels  int
}

type AudioStreamer struct {
	stream      *portaudio.Stream
	audioBuffer []int32
	config      Config
}

func NewAudioStreamer(config Config) *AudioStreamer {
	return &AudioStreamer{
		config:      config,
		audioBuffer: make([]int32, config.FramesPerBuffer),
	}
}

func (a *AudioStreamer) Initialize() error {
	return portaudio.Initialize()
}

func (a *AudioStreamer) Terminate() {
	portaudio.Terminate()
}

func (a *AudioStreamer) Open() error {
	stream, err := portaudio.OpenDefaultStream(
		a.config.InputChannels,
		a.config.OutputChannels,
		a.config.SampleRate,
		a.config.FramesPerBuffer,
		a.audioBuffer,
	)
	if err != nil {
		return err
	}
	a.stream = stream
	return nil
}

func (a *AudioStreamer) Close() error {
	if a.stream != nil {
		return a.stream.Close()
	}
	return nil
}

func (a *AudioStreamer) StartCapture(ctx context.Context, audioData chan<- []byte) error {
	if a.stream == nil {
		return errors.New("Stream not opened")
	}

	if err := a.stream.Start(); err != nil {
		return err
	}
	defer a.stream.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := a.stream.Read(); err != nil {
				log.Printf("Error reading audio: %v", err)
				continue
			}

			// Convert int32 samples to bytes (16-bit PCM)
			audioBytes := a.convertToBytes()

			select {
			case audioData <- audioBytes:
			case <-ctx.Done():
				return ctx.Err()
			default:
				// Drop audio if channel is full
			}
		}
	}
}

func (a *AudioStreamer) convertToBytes() []byte {
	var buf bytes.Buffer
	for _, sample := range a.audioBuffer {
		// Convert 32-bit to 16-bit
		sample16 := int16(sample >> 16)
		binary.Write(&buf, binary.LittleEndian, sample16)
	}
	return buf.Bytes()
}

func GetDefaultConfig() Config {
	return Config{
		SampleRate:      44100,
		FramesPerBuffer: 1024,
		InputChannels:   1,
		OutputChannels:  0,
	}
}
