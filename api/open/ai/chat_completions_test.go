package ai_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/iqhive/runtime.link/api/open/ai"
	"github.com/iqhive/runtime.link/xyz"
)

// realisticCompletion follows the response documented for POST /chat/completions,
// including the fields that this package has no use for.
const realisticCompletion = `{
  "id": "chatcmpl-B9MHDbslfkBeAs8l4bebGdFOJ6PeG",
  "object": "chat.completion",
  "created": 1741570283,
  "model": "gpt-4o-2024-08-06",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "The image shows a wooden boardwalk.",
        "refusal": null,
        "annotations": []
      },
      "logprobs": {
        "content": [
          {"token": "The", "logprob": -0.25, "bytes": [84, 104, 101], "top_logprobs": [
            {"token": "A", "logprob": -1.5, "bytes": [65], "top_logprobs": []}
          ]}
        ],
        "refusal": null
      },
      "finish_reason": "stop"
    },
    {
      "index": 1,
      "message": {
        "role": "assistant",
        "content": null,
        "tool_calls": [
          {
            "id": "call_abc123",
            "type": "function",
            "function": {"name": "get_weather", "arguments": "{\"city\":\"Paris\"}"}
          }
        ]
      },
      "logprobs": null,
      "finish_reason": "tool_calls"
    }
  ],
  "usage": {
    "prompt_tokens": 1117,
    "completion_tokens": 46,
    "total_tokens": 1163,
    "prompt_tokens_details": {"cached_tokens": 3, "audio_tokens": 0},
    "completion_tokens_details": {"reasoning_tokens": 0}
  },
  "service_tier": "default",
  "system_fingerprint": "fp_fc9f1d7035"
}`

func checkRealisticCompletion(t *testing.T, completion ai.ChatCompletion) {
	t.Helper()
	if completion.ID != "chatcmpl-B9MHDbslfkBeAs8l4bebGdFOJ6PeG" || completion.Created != 1741570283 ||
		completion.Model != "gpt-4o-2024-08-06" || completion.SystemFingerprint != "fp_fc9f1d7035" {
		t.Errorf("header: %+v", completion)
	}
	if completion.Usage.TotalTokens != 1163 || completion.Usage.PromptTokensDetails.CachedTokens != 3 {
		t.Errorf("usage: %+v", completion.Usage)
	}
	if len(completion.Choices) != 2 {
		t.Fatalf("got %d choices", len(completion.Choices))
	}

	first := completion.Choices[0]
	if xyz.ValueOf(first.Message) != ai.Messages.Assistant {
		t.Fatalf("first message: %v", xyz.ValueOf(first.Message))
	}
	if got := ai.Messages.Assistant.Get(first.Message).Content; got != "The image shows a wooden boardwalk." {
		t.Errorf("content: %q", got)
	}
	if first.FinishReason != ai.FinishReasons.Stop {
		t.Errorf("finish reason: %v", first.FinishReason)
	}
	logged := first.LoggedProbabilities
	if len(logged.Content) != 1 {
		t.Fatalf("logprobs: %+v", logged)
	}
	token := logged.Content[0]
	if token.Token != "The" || token.Probability != -0.25 || !reflect.DeepEqual(token.Bytes, []rune{84, 104, 101}) {
		t.Errorf("token: %+v", token)
	}
	if len(token.TopProbabilities) != 1 || token.TopProbabilities[0].Probability != -1.5 {
		t.Errorf("top logprobs: %+v", token.TopProbabilities)
	}

	second := completion.Choices[1]
	if second.FinishReason != ai.FinishReasons.ToolCalls {
		t.Errorf("finish reason: %v", second.FinishReason)
	}
	if len(second.LoggedProbabilities.Content) != 0 {
		t.Errorf("logprobs: %+v", second.LoggedProbabilities)
	}
	calls := ai.Messages.Assistant.Get(second.Message).ToolCalls
	if len(calls) != 1 || xyz.ValueOf(calls[0]) != ai.ToolCalls.Function {
		t.Fatalf("tool calls: %v", calls)
	}
	// Without the id there is nothing for the tool's reply to answer.
	call := ai.ToolCalls.Function.Get(calls[0])
	if call.ID != "call_abc123" || call.Function.Name != "get_weather" || call.Function.Arguments != `{"city":"Paris"}` {
		t.Errorf("tool call: %+v", call)
	}
}

func TestChatCompletionRoundTrip(t *testing.T) {
	var completion ai.ChatCompletion
	if err := json.Unmarshal([]byte(realisticCompletion), &completion); err != nil {
		t.Fatal(err)
	}
	checkRealisticCompletion(t, completion)

	encoded, err := json.Marshal(completion)
	if err != nil {
		t.Fatal(err)
	}
	var again ai.ChatCompletion
	if err := json.Unmarshal(encoded, &again); err != nil {
		t.Fatalf("%v in %s", err, encoded)
	}
	checkRealisticCompletion(t, again)
}

func TestLogProbabilitiesLeftOutWhenAbsent(t *testing.T) {
	encoded, err := json.Marshal(ai.Choice{FinishReason: ai.FinishReasons.Stop})
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	if _, ok := fields["logprobs"]; ok {
		t.Errorf("logprobs was not requested: %s", encoded)
	}
}

func TestStreamedChunk(t *testing.T) {
	var chunk ai.ChatCompletionChunk
	body := `{"id":"x","object":"chat.completion.chunk","created":1,"model":"m",
	 "choices":[{"index":0,"delta":{"role":"assistant","content":"h"},"finish_reason":null}]}`
	if err := json.Unmarshal([]byte(body), &chunk); err != nil {
		t.Fatal(err)
	}
	if got := chunk.Choices[0].Delta.Content; got != "h" {
		t.Errorf("got %q", got)
	}
}

// Only the first chunk of a streamed message has a role.
func TestStreamedDeltasWithoutRole(t *testing.T) {
	stream := []string{
		`{"id":"x","object":"chat.completion.chunk","created":1,"model":"m",
		 "choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}`,
		`{"id":"x","object":"chat.completion.chunk","created":1,"model":"m",
		 "choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}`,
		`{"id":"x","object":"chat.completion.chunk","created":1,"model":"m",
		 "choices":[{"index":0,"delta":{"content":" world"},"finish_reason":null}]}`,
		`{"id":"x","object":"chat.completion.chunk","created":1,"model":"m",
		 "choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
	}
	var content string
	for i, body := range stream {
		var chunk ai.ChatCompletionChunk
		if err := json.Unmarshal([]byte(body), &chunk); err != nil {
			t.Fatalf("chunk %d: %v", i, err)
		}
		delta := chunk.Choices[0].Delta
		if i == 0 && delta.Role != ai.Roles.Assistant {
			t.Errorf("first chunk has the role %q", delta.Role)
		}
		content += delta.Content
	}
	if content != "Hello world" {
		t.Errorf("got %q", content)
	}
}

func TestStreamedToolCallDeltas(t *testing.T) {
	stream := []string{
		`{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"get_weather","arguments":""}}]}`,
		`{"tool_calls":[{"index":0,"function":{"arguments":"{\"city\""}}]}`,
		`{"tool_calls":[{"index":0,"function":{"arguments":":\"Paris\"}"}}]}`,
	}
	var id ai.ToolCallID
	var name, arguments string
	for i, body := range stream {
		var delta ai.Delta
		if err := json.Unmarshal([]byte(body), &delta); err != nil {
			t.Fatalf("chunk %d: %v", i, err)
		}
		call := delta.ToolCalls[0]
		if call.ID != "" {
			id = call.ID
		}
		if call.Function.Name != "" {
			name = call.Function.Name
		}
		arguments += call.Function.Arguments
	}
	if id != "call_1" || name != "get_weather" || arguments != `{"city":"Paris"}` {
		t.Errorf("got %q %q %q", id, name, arguments)
	}
}

func TestMessagesRoundTrip(t *testing.T) {
	for _, in := range []string{
		`{"content":"s","role":"system"}`,
		`{"content":"u","role":"user"}`,
		`{"content":"a","role":"assistant"}`,
		`{"tool_call_id":"1","content":"t","role":"tool"}`,
	} {
		var message ai.Message
		if err := json.Unmarshal([]byte(in), &message); err != nil {
			t.Errorf("%s: %v", in, err)
			continue
		}
		out, err := json.Marshal(message)
		if err != nil || string(out) != in {
			t.Errorf("%s became %s, %v", in, out, err)
		}
	}
}

func TestRequestEncoding(t *testing.T) {
	encoded, err := json.Marshal(ai.ChatCompletionRequest{
		Messages:       []ai.Message{ai.Messages.User.As(ai.User{Content: "hi"})},
		Model:          "gpt-4o",
		ResponseFormat: ai.ResponseFormats.JSON,
		Tools:          []ai.Tool{ai.Tools.Function.As(ai.Function{Name: "get_weather"})},
		ToolChoice:     ai.ToolChoices.Specific.As(ai.Function{Name: "get_weather"}),
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"messages":[{"content":"hi","role":"user"}],"model":"gpt-4o",` +
		`"response_format":{"type":"json_object"},` +
		`"tools":[{"function":{"name":"get_weather"},"type":"function"}],` +
		`"tool_choice":{"function":{"name":"get_weather"},"type":"function"}}`
	if string(encoded) != want {
		t.Errorf("got  %s\nwant %s", encoded, want)
	}
}

func TestToolChoiceOnTheWire(t *testing.T) {
	specific := ai.ToolChoices.Specific.As(ai.Function{Name: "get_weather"})
	tests := []struct {
		choice ai.ToolChoice
		want   string
	}{
		{ai.ToolChoices.None, `"none"`},
		{ai.ToolChoices.Auto, `"auto"`},
		{ai.ToolChoices.Required, `"required"`},
		{specific, `{"function":{"name":"get_weather"},"type":"function"}`},
	}
	for _, tt := range tests {
		encoded, err := json.Marshal(tt.choice)
		if err != nil || string(encoded) != tt.want {
			t.Errorf("got %s, %v, want %s", encoded, err, tt.want)
			continue
		}
		var decoded ai.ToolChoice
		if err := json.Unmarshal(encoded, &decoded); err != nil {
			t.Errorf("%s: %v", encoded, err)
			continue
		}
		if decoded != tt.choice {
			t.Errorf("%s decoded as %v", encoded, decoded)
		}
	}
}

// OpenAI documents the keys of the object in this order, not the one sent above.
func TestToolChoiceFromOpenAI(t *testing.T) {
	var choice ai.ToolChoice
	if err := json.Unmarshal([]byte(`{"type":"function","function":{"name":"my_function"}}`), &choice); err != nil {
		t.Fatal(err)
	}
	if xyz.ValueOf(choice) != ai.ToolChoices.Specific || ai.ToolChoices.Specific.Get(choice).Name != "my_function" {
		t.Errorf("decoded as %v", choice)
	}
}

func TestToolChoiceInRequest(t *testing.T) {
	var request ai.ChatCompletionRequest
	if err := json.Unmarshal([]byte(`{"tool_choice":"required"}`), &request); err != nil {
		t.Fatal(err)
	}
	if request.ToolChoice != ai.ToolChoices.Required {
		t.Errorf("decoded as %v", request.ToolChoice)
	}
	encoded, err := json.Marshal(ai.ChatCompletionRequest{Model: "gpt-4o", ToolChoice: ai.ToolChoices.Auto})
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"messages":null,"model":"gpt-4o","tool_choice":"auto"}`; string(encoded) != want {
		t.Errorf("got %s", encoded)
	}
	encoded, err = json.Marshal(ai.ChatCompletionRequest{Model: "gpt-4o"})
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"messages":null,"model":"gpt-4o"}`; string(encoded) != want {
		t.Errorf("an unset choice must be left out: %s", encoded)
	}
}

func TestToolChoiceRejectsOtherShapes(t *testing.T) {
	for _, input := range []string{`{"type":"bogus"}`, `{}`, `{"type":1}`, `1`, `[1]`} {
		choice := ai.ToolChoices.Auto
		if err := json.Unmarshal([]byte(input), &choice); err == nil {
			t.Errorf("%s was accepted", input)
		}
		if choice != ai.ToolChoices.Auto {
			t.Errorf("%s changed the value to %v", input, choice)
		}
	}
}

func TestToolRoundTrip(t *testing.T) {
	encoded, err := json.Marshal(ai.Tools.Function.As(ai.Function{Name: "get_weather", Description: "weather"}))
	if err != nil {
		t.Fatal(err)
	}
	var tool ai.Tool
	if err := json.Unmarshal(encoded, &tool); err != nil {
		t.Fatal(err)
	}
	if xyz.ValueOf(tool) != ai.Tools.Function || ai.Tools.Function.Get(tool).Name != "get_weather" {
		t.Errorf("%s decoded as %v", encoded, xyz.ValueOf(tool))
	}
}
