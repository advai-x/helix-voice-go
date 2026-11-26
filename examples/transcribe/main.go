package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/advai-x/helix-voice-go"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("Usage: go run ./examples/transcribe <audio-file>")
	}

	client, err := helixvoice.NewClient("", helixvoice.ClientOptions{
		BaseURL: os.Getenv("HELIXVOICE_BASE_URL"),
	})
	if err != nil {
		log.Fatal(err)
	}
	result, err := client.CreateTranscription(context.Background(), os.Args[1], "zh-CN")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(result.Text)
}
