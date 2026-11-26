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

	voice := voices[0]
	generation, err := client.SaveSpeech(ctx, "Hello from Helix Voice.", voice.Id, "speech.wav")
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Saved speech.wav with voice %s (generation %s)", voice.Id, generation.Id)
}
