package open_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/iqhive/runtime.link/api"
	"github.com/iqhive/runtime.link/api/open"
	"github.com/iqhive/runtime.link/api/open/ai"
	"github.com/iqhive/runtime.link/api/rest"
	"github.com/iqhive/runtime.link/xyz"
)

// Reaching any test at all shows that the packages can be imported, for their
// package-level accessors run when a binary that imports them starts.
func TestPackagesCanBeImported(t *testing.T) {
	var spec open.AI
	structure := api.StructureOf(&spec)
	if structure.Name != "OpenAI" {
		t.Errorf("got %q", structure.Name)
	}
	chat, ok := structure.Namespace["Chat"]
	if !ok || len(chat.Functions) != 1 {
		t.Fatalf("namespaces: %v", structure.Namespace)
	}
	if chat.Functions[0].Tags.Get("rest") != "POST /chat/completions" {
		t.Errorf("got %q", chat.Functions[0].Tags)
	}
	if ai.Tools.Function.String() != "Function" {
		t.Errorf("got %q", ai.Tools.Function)
	}
}

func TestCreateChatCompletionOverREST(t *testing.T) {
	var request struct {
		Path, Authorization string
		Body                map[string]json.RawMessage
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request.Path = r.URL.Path
		request.Authorization = r.Header.Get("Authorization")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		if err := json.Unmarshal(body, &request.Body); err != nil {
			t.Errorf("%v in %s", err, body)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{
		  "id": "chatcmpl-1", "object": "chat.completion", "created": 1741570283, "model": "gpt-4o",
		  "choices": [{"index": 0, "message": {"role": "assistant", "content": "hi"}, "logprobs": null, "finish_reason": "stop"}],
		  "usage": {"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2}
		}`)
	}))
	defer srv.Close()

	client := api.Import[open.AI](rest.API, srv.URL, ai.Client("key"))
	completion, err := client.Chat.CreateChatCompletion(context.Background(), ai.ChatCompletionRequest{
		Model:    "gpt-4o",
		Messages: []ai.Message{ai.Messages.User.As(ai.User{Content: "hello"})},
	})
	if err != nil {
		t.Fatal(err)
	}
	if request.Path != "/chat/completions" || request.Authorization != "Bearer key" {
		t.Errorf("request: %+v", request)
	}
	var messages bytes.Buffer
	if err := json.Compact(&messages, request.Body["messages"]); err != nil {
		t.Fatal(err)
	}
	if messages.String() != `[{"content":"hello","role":"user"}]` {
		t.Errorf("messages: %s", messages.String())
	}
	if len(completion.Choices) != 1 || xyz.ValueOf(completion.Choices[0].Message) != ai.Messages.Assistant {
		t.Fatalf("got %+v", completion)
	}
	if got := ai.Messages.Assistant.Get(completion.Choices[0].Message).Content; got != "hi" {
		t.Errorf("got %q", got)
	}
}
