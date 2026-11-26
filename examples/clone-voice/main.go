package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/advai-x/helix-voice-go"
)

func main() {
	if len(os.Args) != 6 || os.Args[1] != "--consent" {
		log.Fatal("Usage: go run ./examples/clone-voice --consent <reference-audio> <name> <locale> <transcript>")
	}

	client, err := helixvoice.NewClient("", helixvoice.ClientOptions{
		BaseURL: os.Getenv("HELIXVOICE_BASE_URL"),
	})
	if err != nil {
		log.Fatal(err)
	}
	voice, err := client.CloneVoice(context.Background(), helixvoice.CloneVoiceInput{
		File:       os.Args[2],
		Name:       os.Args[3],
		Language:   os.Args[4],
		Transcript: os.Args[5],
		Consent:    true,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(voice.Id)
}
