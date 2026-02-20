package workspace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sammwy/teragen/internal/ai"
)

const (
	OpCreate = "CREATE"
	OpModify = "MODIFY"
	OpDelete = "DELETE"
	OpMkdir  = "MKDIR"
	OpRmdir  = "RMDIR"
	OpMove   = "MOVE"
)

type FileChange struct {
	Path      string `json:"path"`
	Operation string `json:"operation"`
	Content   string `json:"content,omitempty"` // Content, diff, or destination path
}

type Snapshot struct {
	ID        string       `json:"id"`
	PrevID    string       `json:"prev_id,omitempty"`
	NextID    string       `json:"next_id,omitempty"`
	Timestamp string       `json:"timestamp"`
	Files     []FileChange `json:"files"`
}

type ChatSession struct {
	ID                string       `json:"id"`
	Messages          []ai.Message `json:"messages"`
	TotalInputTokens  int          `json:"total_input_tokens"`
	TotalOutputTokens int          `json:"total_output_tokens"`
	LastSnapshotID    string       `json:"last_snapshot_id,omitempty"`
}

type AgentSession struct {
	ID           string `json:"id"`
	ActiveChatID string `json:"active_chat_id"`
}

type Workspace struct {
	Root string
}

func NewWorkspace(root string) *Workspace {
	absRoot, _ := filepath.Abs(root)
	return &Workspace{Root: absRoot}
}

func (w *Workspace) GetTeragenDir() string {
	return filepath.Join(w.Root, ".teragen")
}

func (w *Workspace) GetChatsDir() string {
	return filepath.Join(w.GetTeragenDir(), "_chats")
}

func (w *Workspace) GetSnapsDir() string {
	return filepath.Join(w.GetTeragenDir(), "_snaps")
}

func (w *Workspace) EnsureDirectories() error {
	dirs := []string{
		w.GetTeragenDir(),
		w.GetChatsDir(),
		w.GetSnapsDir(),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return nil
}

func (w *Workspace) LoadSnapshot(id string) (*Snapshot, error) {
	path := filepath.Join(w.GetSnapsDir(), id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, err
	}
	return &snap, nil
}

func (w *Workspace) SaveSnapshot(snap *Snapshot) error {
	path := filepath.Join(w.GetSnapsDir(), snap.ID+".json")
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
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

func (w *Workspace) ListSnapshots(chatID string) ([]*Snapshot, error) {
	dir := w.GetSnapsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var snaps []*Snapshot
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".json" {
			data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
			if err != nil {
				continue
			}
			var s Snapshot
			if err := json.Unmarshal(data, &s); err == nil {
				snaps = append(snaps, &s)
			}
		}
	}
	return snaps, nil
}
