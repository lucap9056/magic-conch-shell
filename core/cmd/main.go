package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/lucap9056/magic-conch-shell/core/assistant"
	"github.com/lucap9056/magic-conch-shell/core/internal/grpcserver"
	"github.com/lucap9056/magic-conch-shell/core/structs"

	"github.com/lucap9056/go-envfile/envfile"
	"github.com/lucap9056/go-lifecycle/lifecycle"

	"google.golang.org/grpc/keepalive"
)

func main() {
	envfile.Load()

	life := lifecycle.New()

	apiKey := os.Getenv("LLM_API_KEY")
	modelName := os.Getenv("MODEL_NAME")
	allowedImageDomains := os.Getenv("ALLOWED_IMAGE_DOMAINS")
	grpcAddress := os.Getenv("GRPC_ADDRESS")
	grpcCert := os.Getenv("GRPC_TLS_CERT")
	grpcKey := os.Getenv("GRPC_TLS_KEY")
	grpcCA := os.Getenv("GRPC_TLS_CA")
	grpcMaxRecvMsgSize := os.Getenv("GRPC_MAX_RECV_MSG_SIZE")
	grpcMaxSendMsgSize := os.Getenv("GRPC_MAX_SEND_MSG_SIZE")
	grpcKeepaliveTime := os.Getenv("GRPC_KEEPALIVE_TIME")
	grpcKeepaliveTimeout := os.Getenv("GRPC_KEEPALIVE_TIMEOUT")

	serverOptions := []grpcserver.ServerOption{}

	if grpcCert != "" && grpcKey != "" {
		serverOptions = append(serverOptions, grpcserver.WithTLS(grpcCert, grpcKey))
	}
	if grpcCA != "" {
		serverOptions = append(serverOptions, grpcserver.WithCA(grpcCA))
	}

	if grpcMaxRecvMsgSize != "" || grpcMaxSendMsgSize != "" {
		recv := 4 * 1024 * 1024
		send := 4 * 1024 * 1024
		if grpcMaxRecvMsgSize != "" {
			if v, err := strconv.Atoi(grpcMaxRecvMsgSize); err == nil {
				recv = v
			}
		}
		if grpcMaxSendMsgSize != "" {
			if v, err := strconv.Atoi(grpcMaxSendMsgSize); err == nil {
				send = v
			}
		}
		serverOptions = append(serverOptions, grpcserver.WithMaxMsgSize(recv, send))
	}

	if grpcKeepaliveTime != "" || grpcKeepaliveTimeout != "" {
		params := keepalive.ServerParameters{
			Time:    2 * time.Hour,
			Timeout: 20 * time.Second,
		}
		if grpcKeepaliveTime != "" {
			if d, err := time.ParseDuration(grpcKeepaliveTime); err == nil {
				params.Time = d
			}
		}
		if grpcKeepaliveTimeout != "" {
			if d, err := time.ParseDuration(grpcKeepaliveTimeout); err == nil {
				params.Timeout = d
			}
		}
		serverOptions = append(serverOptions, grpcserver.WithKeepalive(params))
	}

	asst, err := assistant.NewClient(apiKey, modelName, allowedImageDomains)
	if err != nil {
		life.Exitln(err)
		return
	}

	if grpcAddress != "" {
		grpcServer, err := grpcserver.NewGRPCServer(asst, serverOptions...)
		if err != nil {
			life.Exitln(err)
			return
		}
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
