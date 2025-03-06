package open

import (
	"context"

	"runtime.link/api"
	"runtime.link/api/open/ai"
)

type AI struct {
	api.Specification `api:"OpenAI"`

	Error api.Register[error, struct {
		ai.Error `json:"error"`
	}]

	Chat struct {
		CreateChatCompletion func(context.Context, ai.ChatCompletionRequest) (ai.ChatCompletion, error) `rest:"POST /chat/completions"
			creates a model response for the given chat conversation.`
	}
}
