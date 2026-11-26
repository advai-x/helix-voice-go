<p align="center">
  <img src="https://docs.helixvoice.net/helix-voice-touch-icon.png" alt="Helix Voice" width="64">
</p>

<h1 align="center">Helix Voice SDK for Go</h1>

<p align="center">
  Generate speech, transcribe audio, and manage voices from your Go application.
</p>

<p align="center">
  <a href="https://pkg.go.dev/github.com/advai-x/helix-voice-go"><img src="https://img.shields.io/github/v/tag/advai-x/helix-voice-go?label=Go%20module&amp;sort=semver" alt="Go module version"></a>
</p>

<p align="center">
  <a href="https://docs.helixvoice.net/">Developer Docs</a> ·
  <a href="examples/">Examples</a> ·
  <a href="https://github.com/Advai-X/helix-voice-go/tags">Version History</a>
</p>

## Install

Requires Go 1.18 or later. Run this command in your Go module. If you are starting a new module, run `go mod init example.com/speech` first.

```sh
go get github.com/advai-x/helix-voice-go@v0.2.0
```

Import `github.com/advai-x/helix-voice-go`. The package name is `helixvoice`.

## Quickstart

Set `HELIXVOICE_API_KEY` to a server-side API key with `voices:read`, `speech:write`, and `generations:read` scopes.

```sh
export HELIXVOICE_API_KEY='YOUR_API_KEY'
```

Save this as `main.go` in your module and run `go run main.go`.

```go
package main

import (
	"context"
	"log"
	"os"

	"github.com/advai-x/helix-voice-go"
)

func main() {
	ctx := context.Background()
	client, err := helixvoice.NewClient("", helixvoice.ClientOptions{
		BaseURL: os.Getenv("HELIXVOICE_BASE_URL"),
	})
	if err != nil {
		log.Fatal(err)
	}

	voices, err := client.ListVoices(ctx)
	if err != nil {
		log.Fatal(err)
	}
	if len(voices) == 0 {
		log.Fatal("No voices are available to this account.")
	}

	generation, err := client.SaveSpeech(ctx, "Hello from Helix Voice.", voices[0].Id, "speech.wav")
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Saved speech.wav (generation %s)", generation.Id)
}
```

On success, `speech.wav` is saved in your current directory and the generation ID is printed. See also the [repository example](examples/speech/main.go).

## API keys and account access

For production use, ask your account administrator for a scoped API key and set `HELIXVOICE_API_KEY` in your server environment. The speech example needs `voices:read` to look up a voice, `speech:write` to generate speech, and `generations:read` to download the WAV. Keep the key in server-side secret storage. [API keys and scopes](https://docs.helixvoice.net/en/api/authentication/)

The examples default to `https://api.helixvoice.net`. Set `HELIXVOICE_BASE_URL` when running them against a trusted staging or test API.

## Capabilities and scopes

| Operation | Required scope |
| --- | --- |
| Read service capabilities and limits | `capabilities:read` |
| List public and account-owned voices | `voices:read` |
| Clone or delete an owned voice | `voices:write` |
| Generate speech metadata | `speech:write` |
| Transcribe a Mandarin audio file | `transcriptions:write` |
| List generations or download WAV audio | `generations:read` |
| Delete generation history | `generations:write` |

Voice cloning requires explicit consent. Consult the capabilities response for the limits of your environment. [Developer API reference](https://docs.helixvoice.net/en/api/speech/)

## Troubleshooting

- **401 Unauthorized** means the key is missing, invalid, expired, or revoked. Check the key used by the running process.
- **403 Forbidden** with `insufficient_scope` means the key lacks a required permission. Check the scopes for each request.
- **429 Too Many Requests** means a quota or rate limit was reached. Honor `Retry-After` when available.
- **Speech generated but WAV saving failed** means you can recover the generation ID from the save error and retry the download while that record is retained. Retrying generation can consume quota again.

Keep the request ID when investigating an API error. [Errors and rate limits](https://docs.helixvoice.net/en/api/errors/) · [Generation downloads](https://docs.helixvoice.net/en/api/speech/#history-and-audio)

## License

Licensed under the [Apache License, Version 2.0](LICENSE).
