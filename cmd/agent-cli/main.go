package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"agent/internal/agent"
	"agent/internal/config"
	"agent/internal/llm"
	"agent/internal/llm/deepseek"
	"agent/internal/tool"
	"agent/internal/tool/calculator"
	"agent/internal/tool/weather"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	provider := deepseek.NewClient(cfg.APIKey, cfg.BaseURL, cfg.Model, &http.Client{})
	registry := tool.NewRegistry()
	if err := registry.Register(weather.New()); err != nil {
		log.Fatal(err)
	}
	if err := registry.Register(calculator.New()); err != nil {
		log.Fatal(err)
	}
	agentService := agent.New(provider, registry, cfg.MaxAgentSteps)
	session := agent.NewSession()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("\n请输入问题：")
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if strings.EqualFold(input, "exit") {
			break
		}

		ctx, cancel := context.WithTimeout(context.Background(), cfg.RequestTimeout)
		printed := false
		err := agentService.RunStream(ctx, session, input, func(chunk llm.StreamChunk) error {
			if chunk.Content != "" {
				if !printed {
					fmt.Print("AI: ")
					printed = true
				}
				fmt.Print(chunk.Content)
			}
			return nil
		})
		cancel()
		if printed {
			fmt.Println()
		}
		if err != nil {
			fmt.Println("运行失败:", err)
		}
	}
	if err := scanner.Err(); err != nil {
		log.Println("读取输入失败:", err)
	}
}
