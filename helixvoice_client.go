package helixvoice

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const BaseURL = "https://api.helixvoice.net"

const (
	maxWAV    = 16 << 20
	maxUpload = 20 << 20
	maxJSON   = 4 << 20
)

type ClientOptions struct {
	BaseURL string
	Timeout time.Duration
}

// HelixVoiceError preserves the HTTP status and server diagnostics.
type HelixVoiceError struct {
	Status     int
	Code       string
	Message    string
	RequestID  string
	RetryAfter string
}

func (e *HelixVoiceError) Error() string { return e.Message }

// SpeechSaveError exposes the generation so its WAV can be recovered by ID.
type SpeechSaveError struct {
	Generation *Generation
	Cause      error
}

func (e *SpeechSaveError) Error() string {
	return "speech was generated but its WAV could not be saved"
}
func (e *SpeechSaveError) Unwrap() error { return e.Cause }

type PreviewResult struct {
	Audio []byte
	Cache string
}

type transport struct {
	base string
	key  string
	http *http.Client
}

func newTransport(options ClientOptions, key string) (*transport, error) {
	base := options.BaseURL
	if base == "" {
		base = BaseURL
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") ||
		parsed.User != nil || parsed.ForceQuery || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("base URL must be an HTTP(S) base URL")
	}
	timeout := options.Timeout
	if timeout < 0 {
		return nil, errors.New("timeout must be positive")
	}
	if timeout == 0 {
		timeout = 180 * time.Second
	}
	return &transport{
		base: strings.TrimRight(parsed.String(), "/"),
		key:  key,
		http: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

func (t *transport) send(ctx context.Context, verb, route, accept, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, verb, t.base+route, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", accept)
	if t.key != "" {
		req.Header.Set("Authorization", "Bearer "+t.key)
	}
	if body != nil {
		req.Header.Set("Content-Type", contentType)
	}
	response, err := t.http.Do(req)
	if err != nil {
		return nil, err
	}
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return response, nil
	}
	defer response.Body.Close()
	failure := responseError(response, fmt.Sprintf("Helix Voice returned HTTP %d", response.StatusCode))
	if mediaType(response) == "application/json" {
		data, err := readLimit(response.Body, 64<<10)
		if err == nil {
			populateError(failure, data)
		}
	}
	return nil, failure
}

func mediaType(response *http.Response) string {
	value, _, _ := mime.ParseMediaType(response.Header.Get("Content-Type"))
	return value
}

func responseError(response *http.Response, message string) *HelixVoiceError {
	return &HelixVoiceError{
		Status: response.StatusCode, Message: message, RequestID: response.Header.Get("X-Request-ID"),
		RetryAfter: response.Header.Get("Retry-After"),
	}
}

type errorDetail struct {
	Code          json.RawMessage
	Message       string
	RequestID     string
	TransactionID string
}

func populateError(failure *HelixVoiceError, data []byte) {
	var payload struct {
		errorDetail
		Error *errorDetail
	}
	if json.Unmarshal(data, &payload) != nil {
		return
	}
	details := payload.errorDetail
	if payload.Error != nil {
		details = *payload.Error
	}
	if details.Message != "" {
		failure.Message = details.Message
	}
	if details.RequestID != "" {
		failure.RequestID = details.RequestID
	} else if details.TransactionID != "" {
		failure.RequestID = details.TransactionID
	}
	if len(details.Code) != 0 && string(details.Code) != "null" {
		if json.Unmarshal(details.Code, &failure.Code) != nil {
			failure.Code = string(details.Code)
		}
	}
}

var errSize = errors.New("response exceeds SDK size limit")

func readLimit(reader io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err == nil && int64(len(data)) > limit {
		err = errSize
	}
	return data, err
}

func fetchJSON[T any](ctx context.Context, t *transport, verb, route, contentType string, body io.Reader) (T, error) {
	var result T
	response, err := t.send(ctx, verb, route, "application/json", contentType, body)
	if err != nil {
		return result, err
	}
	defer response.Body.Close()
	if mediaType(response) == "application/json" {
		data, err := readLimit(response.Body, maxJSON)
		if err != nil && !errors.Is(err, errSize) {
			return result, err
		}
		if err == nil {
			var envelope struct{ Data *T }
			if json.Unmarshal(data, &envelope) == nil && envelope.Data != nil {
				return *envelope.Data, nil
			}
		}
	}
	return result, responseError(response, "Unexpected Helix Voice JSON response")
}

func (t *transport) wav(ctx context.Context, verb, route string, body io.Reader) (PreviewResult, error) {
	response, err := t.send(ctx, verb, route, "audio/wav", "application/json", body)
	if err != nil {
		return PreviewResult{}, err
	}
	defer response.Body.Close()
	if mediaType(response) != "audio/wav" {
		return PreviewResult{}, responseError(response, "Expected a WAV response")
	}
	data, err := readLimit(response.Body, maxWAV)
	if errors.Is(err, errSize) {
		err = responseError(response, "WAV exceeds the SDK response size limit")
	}
	if err != nil {
		return PreviewResult{}, err
	}
	return PreviewResult{Audio: data, Cache: response.Header.Get("X-TTS-Preview-Cache")}, nil
}

func (t *transport) remove(ctx context.Context, route string) error {
	response, err := t.send(ctx, http.MethodDelete, route, "application/json", "", nil)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		return responseError(response, "Expected HTTP 204")
	}
	return nil
}

func resourceID(id string) (string, error) {
	if id == "" || id == "." || id == ".." {
		return "", errors.New("a resource ID is required")
	}
	return url.PathEscape(id), nil
}

func multipartBody(path string, fields map[string]string) (*bytes.Buffer, string, error) {
	source, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer source.Close()
	info, err := source.Stat()
	if err != nil {
		return nil, "", err
	}
	if !info.Mode().IsRegular() || info.Size() > maxUpload {
		return nil, "", errors.New("upload must be a regular file at most 20 MiB")
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for name, value := range fields {
		if err := writer.WriteField(name, value); err != nil {
			return nil, "", err
		}
	}
	file, err := writer.CreateFormFile("file", filepath.Base(path))
	if err != nil {
		return nil, "", err
	}
	size, err := io.Copy(file, io.LimitReader(source, maxUpload+1))
	if err != nil {
		return nil, "", err
	}
	if size > maxUpload {
		return nil, "", errors.New("upload exceeds 20 MiB")
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return &body, writer.FormDataContentType(), nil
}

func writeWAV(path string, audio []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".helixvoice-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	_, err = tmp.Write(audio)
	closeErr := tmp.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(tmp.Name(), path)
}

type Client struct{ http *transport }

func NewClient(apiKey string, options ClientOptions) (*Client, error) {
	if apiKey == "" {
		apiKey = os.Getenv("HELIXVOICE_API_KEY")
	}
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, errors.New("a Helix Voice API key is required")
	}
	http, err := newTransport(options, apiKey)
	if err != nil {
		return nil, err
	}
	return &Client{http: http}, nil
}

func (c *Client) GetCapabilities(ctx context.Context) (*Capabilities, error) {
	return fetchJSON[*Capabilities](ctx, c.http, http.MethodGet, "/v1/capabilities", "", nil)
}

func (c *Client) ListVoices(ctx context.Context) ([]Voice, error) {
	return fetchJSON[[]Voice](ctx, c.http, http.MethodGet, "/v1/voices", "", nil)
}

type CloneVoiceInput struct {
	File       string
	Name       string
	Language   string
	Transcript string
	Consent    bool
}

func (c *Client) CloneVoice(ctx context.Context, input CloneVoiceInput) (*Voice, error) {
	if !input.Consent {
		return nil, errors.New("explicit consent is required to clone a voice")
	}
	body, contentType, err := multipartBody(input.File, map[string]string{
		"name": input.Name, "language": input.Language, "transcript": input.Transcript, "consent": "true",
	})
	if err != nil {
		return nil, err
	}
	return fetchJSON[*Voice](ctx, c.http, http.MethodPost, "/v1/voices", contentType, body)
}

func (c *Client) DeleteVoice(ctx context.Context, id string) error {
	segment, err := resourceID(id)
	if err != nil {
		return err
	}
	return c.http.remove(ctx, "/v1/voices/"+segment)
}

func (c *Client) GenerateSpeech(ctx context.Context, input, voice string) (*Generation, error) {
	payload, err := json.Marshal(NewSpeechRequest(input, voice))
	if err != nil {
		return nil, err
	}
	return fetchJSON[*Generation](ctx, c.http, http.MethodPost, "/v1/audio/speech", "application/json", bytes.NewReader(payload))
}

func (c *Client) SaveSpeech(ctx context.Context, input, voice, output string) (*Generation, error) {
	generation, err := c.GenerateSpeech(ctx, input, voice)
	if err != nil {
		return nil, err
	}
	audio, err := c.DownloadGeneration(ctx, generation.Id)
	if err == nil {
		err = writeWAV(output, audio)
	}
	if err != nil {
		return nil, &SpeechSaveError{Generation: generation, Cause: err}
	}
	return generation, nil
}

func (c *Client) CreateTranscription(ctx context.Context, path, language string) (*Transcription, error) {
	fields := make(map[string]string)
	if language != "" {
		fields["language"] = language
	}
	body, contentType, err := multipartBody(path, fields)
	if err != nil {
		return nil, err
	}
	return fetchJSON[*Transcription](ctx, c.http, http.MethodPost, "/v1/audio/transcriptions", contentType, body)
}

func (c *Client) ListGenerations(ctx context.Context) ([]Generation, error) {
	return fetchJSON[[]Generation](ctx, c.http, http.MethodGet, "/v1/generations", "", nil)
}

func (c *Client) DownloadGeneration(ctx context.Context, id string) ([]byte, error) {
	segment, err := resourceID(id)
	if err != nil {
		return nil, err
	}
	result, err := c.http.wav(ctx, http.MethodGet, "/v1/generations/"+segment+"/audio", nil)
	return result.Audio, err
}

func (c *Client) DeleteGeneration(ctx context.Context, id string) error {
	segment, err := resourceID(id)
	if err != nil {
		return err
	}
	return c.http.remove(ctx, "/v1/generations/"+segment)
}

type PreviewClient struct{ http *transport }

func NewPreviewClient(options ClientOptions) (*PreviewClient, error) {
	http, err := newTransport(options, "")
	if err != nil {
		return nil, err
	}
	return &PreviewClient{http: http}, nil
}

func (c *PreviewClient) Generate(ctx context.Context, input, voice string) (PreviewResult, error) {
	request := NewPreviewRequest(input)
	if voice != "" {
		request.SetVoice(voice)
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return PreviewResult{}, err
	}
	return c.http.wav(ctx, http.MethodPost, "/v1/audio/preview", bytes.NewReader(payload))
}
