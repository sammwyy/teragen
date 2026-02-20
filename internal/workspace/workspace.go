package workspace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sammwy/teragen/internal/ai"
)

type ChatSession struct {
	ID          string       `json:"id"`
	Messages    []ai.Message `json:"messages"`
	TotalTokens int          `json:"total_tokens"`
}

type AgentSession struct {
	ID           string `json:"id"`
	ActiveChatID string `json:"active_chat_id"`
}

type Workspace struct {
	Root string
}

func NewWorkspace(root string) *Workspace {
	return &Workspace{Root: root}
}

func (w *Workspace) GetTeragenDir() string {
	return filepath.Join(w.Root, ".teragen")
}

func (w *Workspace) GetChatsDir() string {
	return filepath.Join(w.GetTeragenDir(), "_chats")
}

func (w *Workspace) EnsureDirectories() error {
	dirs := []string{
		w.GetTeragenDir(),
		w.GetChatsDir(),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return nil
}

func (w *Workspace) LoadChat(chatID string) (*ChatSession, error) {
	path := filepath.Join(w.GetChatsDir(), chatID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var session ChatSession
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}
	return &session, nil
}

func (w *Workspace) SaveChat(session *ChatSession) error {
	path := filepath.Join(w.GetChatsDir(), session.ID+".json")
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (w *Workspace) LoadAgents() ([]AgentSession, error) {
	path := filepath.Join(w.GetTeragenDir(), "_agents.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Return default agent if file doesn't exist
			defaultChatID := "1"
			agents := []AgentSession{{ID: "1", ActiveChatID: defaultChatID}}
			// Ensure initial chat file exists
			w.SaveChat(&ChatSession{ID: defaultChatID, Messages: []ai.Message{}})
			w.SaveAgents(agents)
			return agents, nil
		}
		return nil, err
	}
	var agents []AgentSession
	if err := json.Unmarshal(data, &agents); err != nil {
		return nil, err
	}
	return agents, nil
}

func (w *Workspace) NextChatID() string {
	for i := 1; ; i++ {
		id := fmt.Sprintf("%d", i)
		path := filepath.Join(w.GetChatsDir(), id+".json")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return id
		}
	}
}

func (w *Workspace) DeleteChat(chatID string) error {
	path := filepath.Join(w.GetChatsDir(), chatID+".json")
	return os.Remove(path)
}

func (w *Workspace) SaveAgents(agents []AgentSession) error {
	path := filepath.Join(w.GetTeragenDir(), "_agents.json")
	data, err := json.MarshalIndent(agents, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
