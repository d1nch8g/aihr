package tts

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"

	tts "github.com/yandex-cloud/go-genproto/yandex/cloud/ai/tts/v3"
)

const (
	YandexTTSEndpoint = "tts.api.cloud.yandex.net:443"
)

type YandexConfig struct {
	ApiKey   string
	FolderID string
}

type YandexTTSClient struct {
	client   tts.SynthesizerClient
	conn     *grpc.ClientConn
	apiKey   string
	folderID string
}

// Ensure YandexTTSClient implements Synthesizer interface
var _ Synthesizer = (*YandexTTSClient)(nil)

func GetDefaultSynthesisOptions() SynthesisOptions {
	return SynthesisOptions{
		Voice:                 "marina",
		Speed:                 1.0,
		Volume:                0.0,
		Model:                 "general",
		Format:                tts.ContainerAudio_WAV,
		LoudnessNormalization: tts.UtteranceSynthesisRequest_LUFS,
	}
}

func NewYandexTTSClient(config YandexConfig) (*YandexTTSClient, error) {
	// Create TLS credentials
	creds := credentials.NewTLS(&tls.Config{})

	// Create gRPC connection
	conn, err := grpc.Dial(YandexTTSEndpoint, grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to TTS service: %w", err)
	}

	// Create TTS client
	client := tts.NewSynthesizerClient(conn)

	return &YandexTTSClient{
		client:   client,
		conn:     conn,
		apiKey:   config.ApiKey,
		folderID: config.FolderID,
	}, nil
}

func (c *YandexTTSClient) SynthesizeToStreamWithContext(ctx context.Context, text string, options SynthesisOptions, audioData chan<- []byte) error {
	// Create context with API key and folder ID
	ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Api-Key "+c.apiKey)
	ctx = metadata.AppendToOutgoingContext(ctx, "x-folder-id", c.folderID)

	// Prepare synthesis request
	req := c.buildRequest(text, options)

	// Call synthesis
	stream, err := c.client.UtteranceSynthesis(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to start synthesis: %w", err)
	}

	// Read audio data from stream and send to channel
	defer close(audioData)
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to receive audio data: %w", err)
		}

		// Send audio chunk data to channel
		if audioChunk := resp.GetAudioChunk(); audioChunk != nil {
			select {
			case audioData <- audioChunk.GetData():
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	return nil
}

func (c *YandexTTSClient) buildRequest(text string, options SynthesisOptions) *tts.UtteranceSynthesisRequest {
	req := &tts.UtteranceSynthesisRequest{}

	// Set model
	req.SetModel(options.Model)

	// Set text to synthesize
	req.SetText(text)

	// Set voice hints
	voiceHint := &tts.Hints{}
	voiceHint.SetVoice(options.Voice)

	// Set speed hint
	speedHint := &tts.Hints{}
	speedHint.SetSpeed(options.Speed)

	// Set volume hint
	volumeHint := &tts.Hints{}
	volumeHint.SetVolume(options.Volume)

	// Add hints to request
	req.SetHints([]*tts.Hints{voiceHint, speedHint, volumeHint})

	// Set output audio format
	audioSpec := &tts.AudioFormatOptions{}
	containerAudio := &tts.ContainerAudio{}

	// Type assert the format to Yandex-specific type
	if format, ok := options.Format.(tts.ContainerAudio_ContainerAudioType); ok {
		containerAudio.SetContainerAudioType(format)
	} else {
		containerAudio.SetContainerAudioType(tts.ContainerAudio_WAV) // Default
	}

	audioSpec.SetContainerAudio(containerAudio)
	req.SetOutputAudioSpec(audioSpec)

	// Set loudness normalization
	if normalization, ok := options.LoudnessNormalization.(tts.UtteranceSynthesisRequest_LoudnessNormalizationType); ok {
		req.SetLoudnessNormalizationType(normalization)
	} else {
		req.SetLoudnessNormalizationType(tts.UtteranceSynthesisRequest_LUFS) // Default
	}

	return req
}

func (c *YandexTTSClient) Close() error {
	return c.conn.Close()
}
