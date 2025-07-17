package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"runtime.link/api"
	"runtime.link/api/open"
	"runtime.link/api/open/ai"
	"runtime.link/api/rest"

	_ "embed"
)

var (
	//go:embed prompt.txt
	prompt string

	//go:embed base.txt
	base string
)

func main() {
	ctx := context.Background()
	OpenAI := api.Import[open.AI](rest.API, "https://api.openai.com/v1", ai.Client(os.Getenv("OPENAI_AUTH")))

	var instruction = "ABS"
	if len(os.Args) > 1 {
		instruction = os.Args[1]
	}

	var instructions = make(map[string]string)
	for line := range strings.Lines(base) {
		name, description, _ := strings.Cut(line, ":")
		instructions[name] = description
	}

	desc, ok := instructions[instruction]
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown instruction: %s\n", instruction)
		os.Exit(1)
	}

	instruction = strings.ReplaceAll(instruction, " ", "-")
	desc = strings.ReplaceAll(strings.TrimSpace(desc), " ", "-")

	url := fmt.Sprintf(`https://developer.arm.com/documentation/ddi0602/2024-12/Base-Instructions/%v-%v-?lang=en`, instruction, desc)
	resp, err := http.Get(url)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	result, err := OpenAI.Chat.CreateChatCompletion(ctx, ai.ChatCompletionRequest{
		Model: "gpt-4o",
		Messages: []ai.Message{ai.Messages.User.New(ai.User{
			Content: fmt.Sprintf(prompt, instruction) + string(body),
		})},
	})
	if err != nil {
		panic(err)
	}
	for _, choice := range result.Choices {
		os.Stdout.WriteString(ai.Messages.Assistant.Get(choice.Message).Content)
	}
}
