package agent

import (
	"sync"

	"agent/internal/llm"
)

// Session stores the committed message history for one conversation.
// A session serializes turns so concurrent requests cannot overwrite each other.
type Session struct {
	turnMu     sync.Mutex
	messagesMu sync.RWMutex
	messages   []llm.Message
}

func NewSession() *Session {
	return &Session{}
}

// Messages returns a snapshot that callers may modify safely.
func (s *Session) Messages() []llm.Message {
	s.messagesMu.RLock()
	defer s.messagesMu.RUnlock()
	return cloneMessages(s.messages)
}

// Reset clears the history after any active turn has finished.
func (s *Session) Reset() {
	s.turnMu.Lock()
	defer s.turnMu.Unlock()

	s.messagesMu.Lock()
	s.messages = nil
	s.messagesMu.Unlock()
}

func (s *Session) beginTurn() []llm.Message {
	s.turnMu.Lock()
	return s.Messages()
}

func (s *Session) commitTurn(messages []llm.Message) {
	s.messagesMu.Lock()
	s.messages = cloneMessages(messages)
	s.messagesMu.Unlock()
	s.turnMu.Unlock()
}

func (s *Session) rollbackTurn() {
	s.turnMu.Unlock()
}

func cloneMessages(messages []llm.Message) []llm.Message {
	cloned := make([]llm.Message, len(messages))
	for i, message := range messages {
		cloned[i] = message
		cloned[i].ToolCalls = append([]llm.ToolCall(nil), message.ToolCalls...)
	}
	return cloned
}
