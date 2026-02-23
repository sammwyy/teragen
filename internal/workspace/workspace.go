package workspace

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

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
	Before    string `json:"before,omitempty"`  // Optional: content before change
	After     string `json:"after,omitempty"`   // Optional: content after change
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

type EngineID string

const (
	EngineCwd       EngineID = "Cwd"
	EngineEphemeral EngineID = "Ephemeral"
	EngineMock      EngineID = "Mock"
)

// DirEntry is a minimal directory entry used by workspace engines.
type DirEntry struct {
	Name  string
	IsDir bool
}

// WalkFunc receives a relative path (from workspace root).
type WalkFunc func(rel string, isDir bool) error

// Workspace is the abstraction used by the app for persistence (.teragen) and
// for project filesystem tool operations. Engines may be disk-backed (Cwd) or
// fully in-memory (Ephemeral).
type Workspace interface {
	EngineID() EngineID
	Root() string

	// .teragen locations (virtual for Ephemeral)
	GetTeragenDir() string
	GetChatsDir() string
	GetSnapsDir() string

	// Initialized indicates whether the workspace has ever been persisted
	// (Cwd: .teragen exists; Ephemeral: any write happened).
	Initialized() bool

	// Persistence APIs
	EnsureDirectories() error
	LoadSnapshot(id string) (*Snapshot, error)
	SaveSnapshot(snap *Snapshot) error
	LoadChat(chatID string) (*ChatSession, error)
	SaveChat(session *ChatSession) error
	LoadAgents() ([]AgentSession, error)
	SaveAgents(agents []AgentSession) error
	NextChatID() string
	DeleteChat(chatID string) error
	ListSnapshots(chatID string) ([]*Snapshot, error)

	// Project filesystem APIs (used by evaluator tools/snapshots)
	SafeJoin(target string) (string, error)
	ReadFile(absPath string) ([]byte, error)
	WriteFile(absPath string, data []byte) error
	MkdirAll(absPath string, perm fs.FileMode) error
	ReadDir(absPath string) ([]DirEntry, error)
	Remove(absPath string) error
	RemoveAll(absPath string) error
	Rename(oldAbsPath, newAbsPath string) error
	Walk(relRoot string, fn WalkFunc) error
}

// ─────────────────────────────────────────────────────────────────────────────
// CwdWorkspace (disk-backed)
// ─────────────────────────────────────────────────────────────────────────────

type CwdWorkspace struct {
	root string
}

func NewCwdWorkspace(root string) *CwdWorkspace {
	absRoot, _ := filepath.Abs(root)
	return &CwdWorkspace{root: absRoot}
}

func (w *CwdWorkspace) EngineID() EngineID { return EngineCwd }
func (w *CwdWorkspace) Root() string       { return w.root }

func (w *CwdWorkspace) GetTeragenDir() string { return filepath.Join(w.root, ".teragen") }
func (w *CwdWorkspace) GetChatsDir() string   { return filepath.Join(w.GetTeragenDir(), "_chats") }
func (w *CwdWorkspace) GetSnapsDir() string   { return filepath.Join(w.GetTeragenDir(), "_snaps") }

func (w *CwdWorkspace) Initialized() bool {
	info, err := os.Stat(w.GetTeragenDir())
	return err == nil && info.IsDir()
}

func (w *CwdWorkspace) EnsureDirectories() error {
	dirs := []string{w.GetTeragenDir(), w.GetChatsDir(), w.GetSnapsDir()}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func (w *CwdWorkspace) ensureForWrite() error {
	// NOTE: we intentionally do not create directories on startup.
	return w.EnsureDirectories()
}

func (w *CwdWorkspace) LoadSnapshot(id string) (*Snapshot, error) {
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

func (w *CwdWorkspace) SaveSnapshot(snap *Snapshot) error {
	if err := w.ensureForWrite(); err != nil {
		return err
	}
	path := filepath.Join(w.GetSnapsDir(), snap.ID+".json")
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (w *CwdWorkspace) LoadChat(chatID string) (*ChatSession, error) {
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

func (w *CwdWorkspace) SaveChat(session *ChatSession) error {
	if err := w.ensureForWrite(); err != nil {
		return err
	}
	path := filepath.Join(w.GetChatsDir(), session.ID+".json")
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (w *CwdWorkspace) LoadAgents() ([]AgentSession, error) {
	path := filepath.Join(w.GetTeragenDir(), "_agents.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Important: do NOT create directories/files until the user submits
			// at least one message.
			return []AgentSession{{ID: "1", ActiveChatID: ""}}, nil
		}
		return nil, err
	}
	var agents []AgentSession
	if err := json.Unmarshal(data, &agents); err != nil {
		return nil, err
	}
	return agents, nil
}

func (w *CwdWorkspace) SaveAgents(agents []AgentSession) error {
	if err := w.ensureForWrite(); err != nil {
		return err
	}
	path := filepath.Join(w.GetTeragenDir(), "_agents.json")
	data, err := json.MarshalIndent(agents, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (w *CwdWorkspace) NextChatID() string {
	for i := 1; ; i++ {
		id := fmt.Sprintf("%d", i)
		path := filepath.Join(w.GetChatsDir(), id+".json")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return id
		}
	}
}

func (w *CwdWorkspace) DeleteChat(chatID string) error {
	path := filepath.Join(w.GetChatsDir(), chatID+".json")
	return os.Remove(path)
}

func (w *CwdWorkspace) ListSnapshots(chatID string) ([]*Snapshot, error) {
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

func (w *CwdWorkspace) SafeJoin(target string) (string, error) {
	absRoot, err := filepath.Abs(w.root)
	if err != nil {
		return "", err
	}
	joined := filepath.Join(absRoot, target)
	absJoined, err := filepath.Abs(joined)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(absJoined, absRoot) {
		return "", fmt.Errorf("path traversal attempt: %s", target)
	}
	return absJoined, nil
}

func (w *CwdWorkspace) ReadFile(absPath string) ([]byte, error) {
	return os.ReadFile(absPath)
}

func (w *CwdWorkspace) WriteFile(absPath string, data []byte) error {
	return os.WriteFile(absPath, data, 0o644)
}

func (w *CwdWorkspace) MkdirAll(absPath string, perm fs.FileMode) error {
	return os.MkdirAll(absPath, perm)
}

func (w *CwdWorkspace) ReadDir(absPath string) ([]DirEntry, error) {
	entries, err := os.ReadDir(absPath)
	if err != nil {
		return nil, err
	}
	out := make([]DirEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, DirEntry{Name: e.Name(), IsDir: e.IsDir()})
	}
	return out, nil
}

func (w *CwdWorkspace) Remove(absPath string) error    { return os.Remove(absPath) }
func (w *CwdWorkspace) RemoveAll(absPath string) error { return os.RemoveAll(absPath) }
func (w *CwdWorkspace) Rename(oldAbsPath, newAbsPath string) error {
	return os.Rename(oldAbsPath, newAbsPath)
}

func (w *CwdWorkspace) Walk(relRoot string, fn WalkFunc) error {
	absStart, err := w.SafeJoin(relRoot)
	if err != nil {
		return err
	}
	return filepath.WalkDir(absStart, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(w.root, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		return fn(rel, d.IsDir())
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// EphemeralWorkspace (metadata in-memory, FS on disk)
// ─────────────────────────────────────────────────────────────────────────────

type EphemeralWorkspace struct {
	base        *CwdWorkspace
	mu          sync.RWMutex
	chats       map[string]*ChatSession
	snapshots   map[string]*Snapshot
	agents      []AgentSession
	initialized bool
}

func NewEphemeralWorkspace(base *CwdWorkspace) *EphemeralWorkspace {
	return &EphemeralWorkspace{
		base:      base,
		chats:     make(map[string]*ChatSession),
		snapshots: make(map[string]*Snapshot),
	}
}

func (w *EphemeralWorkspace) EngineID() EngineID { return EngineEphemeral }
func (w *EphemeralWorkspace) Root() string       { return w.base.root }

func (w *EphemeralWorkspace) GetTeragenDir() string { return w.base.GetTeragenDir() }
func (w *EphemeralWorkspace) GetChatsDir() string   { return w.base.GetChatsDir() }
func (w *EphemeralWorkspace) GetSnapsDir() string   { return w.base.GetSnapsDir() }

func (w *EphemeralWorkspace) Initialized() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.initialized
}

func (w *EphemeralWorkspace) EnsureDirectories() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	// Ephemeral: no disk writes; just mark initialized.
	w.initialized = true
	return nil
}

func (w *EphemeralWorkspace) LoadSnapshot(id string) (*Snapshot, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if s, ok := w.snapshots[id]; ok {
		return s, nil
	}
	return nil, os.ErrNotExist
}

func (w *EphemeralWorkspace) SaveSnapshot(snap *Snapshot) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.initialized = true
	cp := *snap
	w.snapshots[snap.ID] = &cp
	return nil
}

func (w *EphemeralWorkspace) LoadChat(chatID string) (*ChatSession, error) {
	w.mu.RLock()
	if c, ok := w.chats[chatID]; ok {
		defer w.mu.RUnlock()
		cp := *c
		return &cp, nil
	}
	w.mu.RUnlock()
	return nil, os.ErrNotExist
}

func (w *EphemeralWorkspace) SaveChat(session *ChatSession) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.initialized = true
	cp := *session
	w.chats[session.ID] = &cp
	return nil
}

func (w *EphemeralWorkspace) LoadAgents() ([]AgentSession, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if len(w.agents) == 0 {
		return []AgentSession{{ID: "1", ActiveChatID: ""}}, nil
	}
	out := make([]AgentSession, len(w.agents))
	copy(out, w.agents)
	return out, nil
}

func (w *EphemeralWorkspace) SaveAgents(agents []AgentSession) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.initialized = true
	w.agents = make([]AgentSession, len(agents))
	copy(w.agents, agents)
	return nil
}

func (w *EphemeralWorkspace) NextChatID() string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	for i := 1; ; i++ {
		id := fmt.Sprintf("%d", i)
		if _, ok := w.chats[id]; !ok {
			return id
		}
	}
}

func (w *EphemeralWorkspace) DeleteChat(chatID string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.chats, chatID)
	return nil
}

func (w *EphemeralWorkspace) ListSnapshots(chatID string) ([]*Snapshot, error) {
	_ = chatID
	w.mu.RLock()
	defer w.mu.RUnlock()
	out := make([]*Snapshot, 0, len(w.snapshots))
	for _, s := range w.snapshots {
		cp := *s
		out = append(out, &cp)
	}
	return out, nil
}

// FS operations delegate to the underlying CwdWorkspace so that LLM tools can
// still work on the real project filesystem.

func (w *EphemeralWorkspace) SafeJoin(target string) (string, error) {
	return w.base.SafeJoin(target)
}

func (w *EphemeralWorkspace) ReadFile(absPath string) ([]byte, error) {
	return w.base.ReadFile(absPath)
}

func (w *EphemeralWorkspace) WriteFile(absPath string, data []byte) error {
	return w.base.WriteFile(absPath, data)
}

func (w *EphemeralWorkspace) MkdirAll(absPath string, perm fs.FileMode) error {
	return w.base.MkdirAll(absPath, perm)
}

func (w *EphemeralWorkspace) ReadDir(absPath string) ([]DirEntry, error) {
	return w.base.ReadDir(absPath)
}

func (w *EphemeralWorkspace) Remove(absPath string) error {
	return w.base.Remove(absPath)
}

func (w *EphemeralWorkspace) RemoveAll(absPath string) error {
	return w.base.RemoveAll(absPath)
}

func (w *EphemeralWorkspace) Rename(oldAbsPath, newAbsPath string) error {
	return w.base.Rename(oldAbsPath, newAbsPath)
}

func (w *EphemeralWorkspace) Walk(relRoot string, fn WalkFunc) error {
	return w.base.Walk(relRoot, fn)
}

// ─────────────────────────────────────────────────────────────────────────────
// MemoryWorkspace (full in-memory FS, used for mock mode)
// ─────────────────────────────────────────────────────────────────────────────

type MemoryWorkspace struct {
	root        string
	mu          sync.RWMutex
	files       map[string][]byte
	dirs        map[string]struct{}
	initialized bool
}

func NewMemoryWorkspace(root string) *MemoryWorkspace {
	absRoot, _ := filepath.Abs(root)
	w := &MemoryWorkspace{
		root:  absRoot,
		files: make(map[string][]byte),
		dirs:  make(map[string]struct{}),
	}
	// Root always exists.
	w.dirs[absRoot] = struct{}{}
	return w
}

func (w *MemoryWorkspace) EngineID() EngineID { return EngineMock }
func (w *MemoryWorkspace) Root() string       { return w.root }

func (w *MemoryWorkspace) GetTeragenDir() string { return filepath.Join(w.root, ".teragen") }
func (w *MemoryWorkspace) GetChatsDir() string   { return filepath.Join(w.GetTeragenDir(), "_chats") }
func (w *MemoryWorkspace) GetSnapsDir() string   { return filepath.Join(w.GetTeragenDir(), "_snaps") }

func (w *MemoryWorkspace) Initialized() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.initialized
}

func (w *MemoryWorkspace) EnsureDirectories() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.initialized = true
	// Virtual dirs
	w.dirs[w.GetTeragenDir()] = struct{}{}
	w.dirs[w.GetChatsDir()] = struct{}{}
	w.dirs[w.GetSnapsDir()] = struct{}{}
	return nil
}

func (w *MemoryWorkspace) SafeJoin(target string) (string, error) {
	absRoot, err := filepath.Abs(w.root)
	if err != nil {
		return "", err
	}
	joined := filepath.Join(absRoot, target)
	absJoined, err := filepath.Abs(joined)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(absJoined, absRoot) {
		return "", fmt.Errorf("path traversal attempt: %s", target)
	}
	return absJoined, nil
}

func (w *MemoryWorkspace) ReadFile(absPath string) ([]byte, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if b, ok := w.files[absPath]; ok {
		out := make([]byte, len(b))
		copy(out, b)
		return out, nil
	}
	return nil, os.ErrNotExist
}

func (w *MemoryWorkspace) WriteFile(absPath string, data []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.initialized = true
	parent := filepath.Dir(absPath)
	w.mkdirAllLocked(parent)
	cp := make([]byte, len(data))
	copy(cp, data)
	w.files[absPath] = cp
	return nil
}

func (w *MemoryWorkspace) MkdirAll(absPath string, perm fs.FileMode) error {
	_ = perm
	w.mu.Lock()
	defer w.mu.Unlock()
	w.initialized = true
	w.mkdirAllLocked(absPath)
	return nil
}

func (w *MemoryWorkspace) mkdirAllLocked(absPath string) {
	p := absPath
	for {
		w.dirs[p] = struct{}{}
		if p == w.root {
			return
		}
		next := filepath.Dir(p)
		if next == p {
			return
		}
		p = next
	}
}

func (w *MemoryWorkspace) ReadDir(absPath string) ([]DirEntry, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if _, ok := w.dirs[absPath]; !ok {
		return nil, os.ErrNotExist
	}
	prefix := absPath
	if !strings.HasSuffix(prefix, string(os.PathSeparator)) {
		prefix += string(os.PathSeparator)
	}

	seen := map[string]DirEntry{}

	for d := range w.dirs {
		if !strings.HasPrefix(d, prefix) {
			continue
		}
		rest := strings.TrimPrefix(d, prefix)
		if rest == "" {
			continue
		}
		name := strings.Split(rest, string(os.PathSeparator))[0]
		if name == "" {
			continue
		}
		seen[name] = DirEntry{Name: name, IsDir: true}
	}
	for f := range w.files {
		if !strings.HasPrefix(f, prefix) {
			continue
		}
		rest := strings.TrimPrefix(f, prefix)
		if rest == "" {
			continue
		}
		name := strings.Split(rest, string(os.PathSeparator))[0]
		if name == "" {
			continue
		}
		if existing, ok := seen[name]; ok && existing.IsDir {
			continue
		}
		seen[name] = DirEntry{Name: name, IsDir: false}
	}

	out := make([]DirEntry, 0, len(seen))
	for _, v := range seen {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (w *MemoryWorkspace) Remove(absPath string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.initialized = true
	if _, ok := w.files[absPath]; ok {
		delete(w.files, absPath)
		return nil
	}
	// removing a dir via Remove is not supported in tools; mimic os.Remove behavior
	if _, ok := w.dirs[absPath]; ok {
		// Directory must be empty
		prefix := absPath
		if !strings.HasSuffix(prefix, string(os.PathSeparator)) {
			prefix += string(os.PathSeparator)
		}
		for d := range w.dirs {
			if strings.HasPrefix(d, prefix) {
				return fmt.Errorf("directory not empty")
			}
		}
		for f := range w.files {
			if strings.HasPrefix(f, prefix) {
				return fmt.Errorf("directory not empty")
			}
		}
		delete(w.dirs, absPath)
		return nil
	}
	return os.ErrNotExist
}

func (w *MemoryWorkspace) RemoveAll(absPath string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.initialized = true

	// delete file
	delete(w.files, absPath)

	// delete directory tree
	prefix := absPath
	if !strings.HasSuffix(prefix, string(os.PathSeparator)) {
		prefix += string(os.PathSeparator)
	}
	for p := range w.files {
		if strings.HasPrefix(p, prefix) {
			delete(w.files, p)
		}
	}
	for d := range w.dirs {
		if d == absPath || strings.HasPrefix(d, prefix) {
			delete(w.dirs, d)
		}
	}
	return nil
}

func (w *MemoryWorkspace) Rename(oldAbsPath, newAbsPath string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.initialized = true
	// file
	if b, ok := w.files[oldAbsPath]; ok {
		delete(w.files, oldAbsPath)
		cp := make([]byte, len(b))
		copy(cp, b)
		w.mkdirAllLocked(filepath.Dir(newAbsPath))
		w.files[newAbsPath] = cp
		return nil
	}
	// dir
	if _, ok := w.dirs[oldAbsPath]; ok {
		oldPrefix := oldAbsPath
		newPrefix := newAbsPath
		if !strings.HasSuffix(oldPrefix, string(os.PathSeparator)) {
			oldPrefix += string(os.PathSeparator)
		}
		if !strings.HasSuffix(newPrefix, string(os.PathSeparator)) {
			newPrefix += string(os.PathSeparator)
		}

		// move dirs
		toMoveDirs := []string{}
		for d := range w.dirs {
			if d == oldAbsPath || strings.HasPrefix(d, oldPrefix) {
				toMoveDirs = append(toMoveDirs, d)
			}
		}
		sort.Strings(toMoveDirs)
		for _, d := range toMoveDirs {
			delete(w.dirs, d)
			rest := strings.TrimPrefix(d, oldAbsPath)
			w.dirs[newAbsPath+rest] = struct{}{}
		}

		// move files
		toMoveFiles := []string{}
		for f := range w.files {
			if strings.HasPrefix(f, oldPrefix) {
				toMoveFiles = append(toMoveFiles, f)
			}
		}
		sort.Strings(toMoveFiles)
		for _, f := range toMoveFiles {
			b := w.files[f]
			delete(w.files, f)
			rest := strings.TrimPrefix(f, oldPrefix)
			w.files[newPrefix+rest] = b
		}

		return nil
	}
	return os.ErrNotExist
}

func (w *MemoryWorkspace) Walk(relRoot string, fn WalkFunc) error {
	absStart, err := w.SafeJoin(relRoot)
	if err != nil {
		return err
	}

	w.mu.RLock()
	defer w.mu.RUnlock()

	paths := make([]string, 0, len(w.dirs)+len(w.files))
	for d := range w.dirs {
		paths = append(paths, d)
	}
	for f := range w.files {
		paths = append(paths, f)
	}
	sort.Strings(paths)

	for _, p := range paths {
		if !strings.HasPrefix(p, absStart) {
			continue
		}
		rel, err := filepath.Rel(w.root, p)
		if err != nil {
			continue
		}
		rel = filepath.ToSlash(rel)
		_, isDir := w.dirs[p]
		if err := fn(rel, isDir); err != nil {
			return err
		}
	}
	return nil
}

func (w *MemoryWorkspace) LoadSnapshot(id string) (*Snapshot, error) {
	path := filepath.Join(w.GetSnapsDir(), id+".json")
	data, err := w.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, err
	}
	return &snap, nil
}

func (w *MemoryWorkspace) SaveSnapshot(snap *Snapshot) error {
	if err := w.EnsureDirectories(); err != nil {
		return err
	}
	path := filepath.Join(w.GetSnapsDir(), snap.ID+".json")
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	return w.WriteFile(path, data)
}

func (w *MemoryWorkspace) LoadChat(chatID string) (*ChatSession, error) {
	path := filepath.Join(w.GetChatsDir(), chatID+".json")
	data, err := w.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var session ChatSession
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}
	return &session, nil
}

func (w *MemoryWorkspace) SaveChat(session *ChatSession) error {
	if err := w.EnsureDirectories(); err != nil {
		return err
	}
	path := filepath.Join(w.GetChatsDir(), session.ID+".json")
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}
	return w.WriteFile(path, data)
}

func (w *MemoryWorkspace) LoadAgents() ([]AgentSession, error) {
	path := filepath.Join(w.GetTeragenDir(), "_agents.json")
	data, err := w.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []AgentSession{{ID: "1", ActiveChatID: ""}}, nil
		}
		return nil, err
	}
	var agents []AgentSession
	if err := json.Unmarshal(data, &agents); err != nil {
		return nil, err
	}
	return agents, nil
}

func (w *MemoryWorkspace) SaveAgents(agents []AgentSession) error {
	if err := w.EnsureDirectories(); err != nil {
		return err
	}
	path := filepath.Join(w.GetTeragenDir(), "_agents.json")
	data, err := json.MarshalIndent(agents, "", "  ")
	if err != nil {
		return err
	}
	return w.WriteFile(path, data)
}

func (w *MemoryWorkspace) NextChatID() string {
	for i := 1; ; i++ {
		id := fmt.Sprintf("%d", i)
		path := filepath.Join(w.GetChatsDir(), id+".json")
		if _, err := w.ReadFile(path); err != nil {
			return id
		}
	}
}

func (w *MemoryWorkspace) DeleteChat(chatID string) error {
	path := filepath.Join(w.GetChatsDir(), chatID+".json")
	return w.Remove(path)
}

func (w *MemoryWorkspace) ListSnapshots(chatID string) ([]*Snapshot, error) {
	_ = chatID
	dir := w.GetSnapsDir()
	entries, err := w.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var snaps []*Snapshot
	for _, entry := range entries {
		if filepath.Ext(entry.Name) != ".json" {
			continue
		}
		data, err := w.ReadFile(filepath.Join(dir, entry.Name))
		if err != nil {
			continue
		}
		var s Snapshot
		if err := json.Unmarshal(data, &s); err == nil {
			snaps = append(snaps, &s)
		}
	}
	return snaps, nil
}
