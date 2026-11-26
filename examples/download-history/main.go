package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/advai-x/helix-voice-go"
)

func main() {
	if len(os.Args) > 2 {
		log.Fatal("Usage: go run ./examples/download-history [generation-id]")
	}

	ctx := context.Background()
	client, err := helixvoice.NewClient("", helixvoice.ClientOptions{
		BaseURL: os.Getenv("HELIXVOICE_BASE_URL"),
	})
	if err != nil {
		log.Fatal(err)
	}
	if len(os.Args) == 1 {
		generations, err := client.ListGenerations(ctx)
		if err != nil {
			log.Fatal(err)
		}
		for _, generation := range generations {
			fmt.Println(generation.Id)
		}
		return
	}

	audio, err := client.DownloadGeneration(ctx, os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile("history.wav", audio, 0o600); err != nil {
		log.Fatal(err)
	}
	fmt.Println("Saved history.wav")
}
