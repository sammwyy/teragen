package evaluator

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/sammwy/teragen/internal/workspace"
)

type SnapshotSession struct {
	ID        string
	ChatID    string
	StartTime time.Time
	Originals map[string]string // Path -> TempPath
	Workspace *workspace.Workspace
}

func (e *Evaluator) StartSnapshotSession(chatID string) *SnapshotSession {
	return &SnapshotSession{
		ID:        uuid.New().String(),
		ChatID:    chatID,
		StartTime: time.Now(),
		Originals: make(map[string]string),
		Workspace: e.Workspace,
	}
}

func (s *SnapshotSession) RecordFile(path string) error {
	if _, ok := s.Originals[path]; ok {
		return nil
	}

	tmpDir := filepath.Join(s.Workspace.GetTeragenDir(), "_tmp", s.ID)
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return err
	}

	// rel, _ := filepath.Rel(s.Workspace.Root, path)
	tmpPath := filepath.Join(tmpDir, uuid.New().String())

	// Copy original to tmp
	src, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, record that "original" was nothing
			s.Originals[path] = ""
			return nil
		}
		return err
	}
	defer src.Close()

	dst, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return err
	}

	s.Originals[path] = tmpPath
	return nil
}

func (s *SnapshotSession) Commit(prevID string) (*workspace.Snapshot, error) {
	snap := &workspace.Snapshot{
		ID:        s.ID,
		PrevID:    prevID,
		Timestamp: s.StartTime.Format(time.RFC3339),
		Files:     []workspace.FileChange{},
	}

	for originalPath, tmpPath := range s.Originals {
		diff := ""
		if tmpPath != "" {
			// Calculate diff between tmpPath (old) and originalPath (new)
			// Using sh -c "diff -u old new"
			cmd := exec.Command("diff", "-u", tmpPath, originalPath)
			output, _ := cmd.CombinedOutput()
			diff = string(output)
		} else {
			// New file
			content, _ := os.ReadFile(originalPath)
			diff = "NEW FILE:\n" + string(content)
		}

		rel, _ := filepath.Rel(s.Workspace.Root, originalPath)
		snap.Files = append(snap.Files, workspace.FileChange{
			Path: rel,
			Diff: diff,
		})
	}

	// Clean up tmp
	tmpDir := filepath.Join(s.Workspace.GetTeragenDir(), "_tmp", s.ID)
	os.RemoveAll(tmpDir)

	if err := s.Workspace.SaveSnapshot(snap); err != nil {
		return nil, err
	}

	return snap, nil
}
