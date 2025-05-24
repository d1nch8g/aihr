package stt

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"

	speechkit "github.com/yandex-cloud/go-genproto/yandex/cloud/ai/stt/v3"
)

type YandexSTTClient struct {
	client   speechkit.RecognizerClient
	conn     *grpc.ClientConn
	iamToken string
	folderID string
	language string
}

type YandexConfig struct {
	IamToken   string
	FolderID   string
	Language   string
	SampleRate int32
}

func NewYandexSTTClient(config YandexConfig) (*YandexSTTClient, error) {
	tlsConfig := &tls.Config{}
	conn, err := grpc.Dial("stt.api.cloud.yandex.net:443", grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Yandex STT: %w", err)
	}

	client := speechkit.NewRecognizerClient(conn)

	return &YandexSTTClient{
		client:   client,
		conn:     conn,
		iamToken: config.IamToken,
		folderID: config.FolderID,
		language: config.Language,
	}, nil
}

func (s *YandexSTTClient) Close() error {
	return s.conn.Close()
}

func (s *YandexSTTClient) StreamRecognize(ctx context.Context, audioData <-chan []byte, results chan<- string, sampleRate int64) error {
	// Create metadata with authorization
	md := metadata.Pairs(
		"authorization", "Bearer "+s.iamToken,
		"x-folder-id", s.folderID,
	)
	ctx = metadata.NewOutgoingContext(ctx, md)

	// Create streaming client
	stream, err := s.client.RecognizeStreaming(ctx)
	if err != nil {
		return fmt.Errorf("failed to create streaming client: %w", err)
	}
	defer stream.CloseSend()

	// Send session options
	sessionOptions := &speechkit.StreamingRequest{
		Event: &speechkit.StreamingRequest_SessionOptions{
			SessionOptions: &speechkit.StreamingOptions{
				RecognitionModel: &speechkit.RecognitionModelOptions{
					AudioFormat: &speechkit.AudioFormatOptions{
						AudioFormat: &speechkit.AudioFormatOptions_RawAudio{
							RawAudio: &speechkit.RawAudio{
								AudioEncoding:     speechkit.RawAudio_LINEAR16_PCM,
								SampleRateHertz:   sampleRate,
								AudioChannelCount: 1,
							},
						},
					},
					TextNormalization: &speechkit.TextNormalizationOptions{
						TextNormalization: speechkit.TextNormalizationOptions_TEXT_NORMALIZATION_ENABLED,
						ProfanityFilter:   false,
						LiteratureText:    false,
					},
					LanguageRestriction: &speechkit.LanguageRestrictionOptions{
						RestrictionType: speechkit.LanguageRestrictionOptions_WHITELIST,
						LanguageCode:    []string{s.language},
					},
					AudioProcessingType: speechkit.RecognitionModelOptions_REAL_TIME,
				},
			},
		},
	}

	if err := stream.Send(sessionOptions); err != nil {
		return fmt.Errorf("failed to send session options: %w", err)
	}

	// Start goroutine to handle responses
	go func() {
		defer close(results)
		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				return
			}
			if err != nil {
				log.Printf("Error receiving response: %v", err)
				return
			}

			if resp.GetFinal() != nil {
				for _, alternative := range resp.GetFinal().GetAlternatives() {
					if text := alternative.GetText(); text != "" {
						results <- text
					}
				}
			}
		}
	}()

	// Send audio data
	for audioChunk := range audioData {
		audioRequest := &speechkit.StreamingRequest{
			Event: &speechkit.StreamingRequest_Chunk{
				Chunk: &speechkit.AudioChunk{
					Data: audioChunk,
				},
			},
		}

		if err := stream.Send(audioRequest); err != nil {
			return fmt.Errorf("failed to send audio chunk: %w", err)
		}
	}

	return nil
}
