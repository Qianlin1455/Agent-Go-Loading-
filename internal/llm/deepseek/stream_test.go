package deepseek

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent/internal/llm"
)

func TestChatStream(t *testing.T) {
	body := readSSEFixture(t, "stream_text.txt")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, body)
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

func TestChatStreamMalformedJSON(t *testing.T) {
	body := readSSEFixture(t, "stream_malformed.txt")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, body)
	}))
	defer server.Close()

	client := NewClient("key", server.URL, "model", server.Client())
	stream, err := client.ChatStream(context.Background(), []llm.Message{{Role: llm.RoleUser, Content: "hi"}}, nil)
	if err != nil {
		t.Fatal(err)
	}

	streamErr := receiveStreamError(stream)
	if streamErr == nil || !strings.Contains(streamErr.Error(), "decode SSE event") {
		t.Fatalf("got error %v, want SSE decode error", streamErr)
	}
}

func TestChatStreamUnexpectedEOF(t *testing.T) {
	body := readSSEFixture(t, "stream_interrupted.txt")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, body)
	}))
	defer server.Close()

	client := NewClient("key", server.URL, "model", server.Client())
	stream, err := client.ChatStream(context.Background(), []llm.Message{{Role: llm.RoleUser, Content: "hi"}}, nil)
	if err != nil {
		t.Fatal(err)
	}

	streamErr := receiveStreamError(stream)
	if !errors.Is(streamErr, io.ErrUnexpectedEOF) {
		t.Fatalf("got error %v, want %v", streamErr, io.ErrUnexpectedEOF)
	}
}

func readSSEFixture(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join("..", "..", "..", "testdata", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(data)
}

func receiveStreamError(stream <-chan llm.StreamChunk) error {
	var streamErr error
	for chunk := range stream {
		if chunk.Err != nil {
			streamErr = chunk.Err
		}
	}
	return streamErr
}
