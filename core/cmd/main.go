package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/lucap9056/magic-conch-shell/core/assistant"
	"github.com/lucap9056/magic-conch-shell/core/internal/grpcserver"
	"github.com/lucap9056/magic-conch-shell/core/structs"

	"github.com/lucap9056/go-envfile/envfile"
	"github.com/lucap9056/go-lifecycle/lifecycle"
)

func main() {
	envfile.Load()

	life := lifecycle.New()

	apiKey := os.Getenv("LLM_API_KEY")
	modelName := os.Getenv("MODEL_NAME")
	allowedImageDomains := os.Getenv("ALLOWED_IMAGE_DOMAINS")
	grpcAddress := os.Getenv("GRPC_ADDRESS")

	asst, err := assistant.NewClient(apiKey, modelName, allowedImageDomains)
	if err != nil {
		life.Exitln(err)
		return
	}

	if grpcAddress != "" {
		grpcServer := grpcserver.NewGRPCServer(asst)
		go func() {
			err := grpcServer.Run(grpcAddress)
			if err != nil {
				log.Printf("[gRPC Server] Error: %v\n", err)
				life.Exitln(err)
			}
		}()
		defer grpcServer.Stop()
	}

	{
		result, err := assistantTest(asst)
		if err != nil {
			life.Exitln(err)
			return
		}

		log.Println("LLM test successful, response:", result)
	}

	go runConsoleMode(asst, life)

	life.Wait()
}

func runConsoleMode(asst *assistant.Client, life *lifecycle.LifecycleManager) {
	fmt.Println("--- Console Mode Enabled (Type 'exit' or 'quit' to stop) ---")

	inputChan := make(chan string)
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			inputChan <- scanner.Text()
		}
		if err := scanner.Err(); err != nil {
			log.Printf("[Console] Stdin scan error: %v\n", err)
		}
		close(inputChan)
	}()

	for {
		fmt.Print("> ")

		select {
		case input, ok := <-inputChan:
			if !ok {
				life.Exitln("Stdin closed.")
				return
			}
			if input == "exit" || input == "quit" {
				fmt.Println("[Console] Exit command received. Shutting down...")
				life.Exitln("User terminated the session via console command.")
				return
			}

			if input == "" {
				continue
			}

			func(text string) {
				reqCtx, cancel := context.WithTimeout(context.Background(), time.Second*10)
				defer cancel()

				go func() {
					select {
					case <-life.Done():
						cancel()
					case <-reqCtx.Done():
					}
				}()

				message := &structs.PromptMessage{
					Parts: []*structs.PromptPart{structs.NewTextPart(text)},
				}
				res, err := asst.GenerateResponse(reqCtx, message, nil)
				if err != nil {
					if errors.Is(err, context.Canceled) {
						return
					}
					fmt.Printf("\n[Error]: %v\n", err)
					return
				}
				fmt.Printf("[MagicConchShell]: %s\n", res)
			}(input)

		case <-life.Done():
			fmt.Println("\n[Console] Termination signal received. Closing...")
			return
		}
	}
}

func assistantTest(llm *assistant.Client) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	message := &structs.PromptMessage{
		Parts: []*structs.PromptPart{structs.NewTextPart("Ok or Fail")},
	}
	reply, err := llm.GenerateResponse(ctx, message, nil)
	if err != nil {
		return "", fmt.Errorf("Assistant Post error: %w", err)
	}

	return reply, nil
}
