package deepseek

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"agent/internal/llm"
)

func TestChatStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"你\"},\"finish_reason\":null}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"好\"},\"finish_reason\":\"stop\"}]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	client := NewClient("key", server.URL, "model", server.Client())
	stream, err := client.ChatStream(context.Background(), []llm.Message{{Role: llm.RoleUser, Content: "hi"}}, nil)
	if err != nil {
		t.Fatal(err)
	}

	var content strings.Builder
	for chunk := range stream {
		if chunk.Err != nil {
			t.Fatal(chunk.Err)
		}
		content.WriteString(chunk.Content)
	}
	if content.String() != "你好" {
		t.Fatalf("unexpected content: %s", content.String())
	}
}
