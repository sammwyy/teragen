package evaluator

import (
	"fmt"
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
	Originals map[string]string               // Path -> TempPath
	Changes   map[string]workspace.FileChange // Path -> Operation Info
	Workspace *workspace.Workspace
}

func (e *Evaluator) StartSnapshotSession(chatID string) *SnapshotSession {
	return &SnapshotSession{
		ID:        uuid.New().String(),
		ChatID:    chatID,
		StartTime: time.Now(),
		Originals: make(map[string]string),
		Changes:   make(map[string]workspace.FileChange),
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

	tmpPath := filepath.Join(tmpDir, uuid.New().String())

	// Copy original to tmp
	src, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
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

func (s *SnapshotSession) RecordOp(path, op string, newPath ...string) {
	fc := workspace.FileChange{
		Path:      path,
		Operation: op,
	}
	if op == workspace.OpMove && len(newPath) > 0 {
		fc.Content = newPath[0]
	}
	s.Changes[path] = fc
}

func (s *SnapshotSession) Commit(prevID string) (*workspace.Snapshot, error) {
	snap := &workspace.Snapshot{
		ID:        s.ID,
		PrevID:    prevID,
		Timestamp: s.StartTime.Format(time.RFC3339),
		Files:     []workspace.FileChange{},
	}

	// We use a map to deduplicate final results by path.
	// We'll iterate through all recorded originals and all explicit ops.
	finalChanges := make(map[string]workspace.FileChange)

	absRoot, _ := filepath.Abs(s.Workspace.Root)

	// Process all files that were modified or might have been changed
	for originalPath, tmpPath := range s.Originals {
		rel, _ := filepath.Rel(absRoot, originalPath)
		change, hasExplicitOp := s.Changes[originalPath]

		op := workspace.OpModify
		content := ""

		if hasExplicitOp {
			op = change.Operation
			if op == workspace.OpMove {
				relNew, _ := filepath.Rel(absRoot, change.Content)
				content = relNew
			}
		} else if tmpPath == "" {
			op = workspace.OpCreate
		}

		if op == workspace.OpModify || op == workspace.OpCreate {
			data, _ := os.ReadFile(originalPath)
			content = string(data)
			if op == workspace.OpModify && tmpPath != "" {
				cmd := exec.Command("diff", "-u", tmpPath, originalPath)
				output, _ := cmd.CombinedOutput()
				content = string(output)
			}
		}

		finalChanges[rel] = workspace.FileChange{
			Path:      rel,
			Operation: op,
			Content:   content,
		}
	}

	// Process explicit operations that didn't have an original file (like MKDIR or CREATE on non-existing path)
	for path, change := range s.Changes {
		rel, _ := filepath.Rel(absRoot, path)
		if _, ok := finalChanges[rel]; ok {
			continue
		}

		op := change.Operation
		content := change.Content // Might be new path for MOVE

		if op == workspace.OpMove {
			relNew, _ := filepath.Rel(absRoot, content)
			content = relNew
		} else if op == workspace.OpCreate || op == workspace.OpModify {
			data, _ := os.ReadFile(path)
			content = string(data)
		}

		finalChanges[rel] = workspace.FileChange{
			Path:      rel,
			Operation: op,
			Content:   content,
		}
	}

	for _, fc := range finalChanges {
		snap.Files = append(snap.Files, fc)
	}

	// Clean up tmp
	tmpDir := filepath.Join(s.Workspace.GetTeragenDir(), "_tmp", s.ID)
	os.RemoveAll(tmpDir)

	if err := s.Workspace.SaveSnapshot(snap); err != nil {
		return nil, err
	}

	return snap, nil
}

func (e *Evaluator) ApplySnapshot(snap *workspace.Snapshot, reverse bool) error {
	files := snap.Files
	if reverse {
		// Apply in reverse order for rollback
		files = make([]workspace.FileChange, len(snap.Files))
		copy(files, snap.Files)
		for i, j := 0, len(files)-1; i < j; i, j = i+1, j-1 {
			files[i], files[j] = files[j], files[i]
		}
	}

	for _, fc := range files {
		absPath, err := e.SafeJoin(fc.Path)
		if err != nil {
			return err
		}

		op := fc.Operation
		if reverse {
			switch op {
			case workspace.OpCreate:
				os.Remove(absPath)
				continue
			case workspace.OpDelete:
				// We can't easily restore deleted files if we didn't save their content
				continue
			case workspace.OpMkdir:
				os.Remove(absPath)
				continue
			case workspace.OpRmdir:
				continue
			case workspace.OpMove:
				absNew, _ := e.SafeJoin(fc.Content)
				os.Rename(absNew, absPath)
				continue
			case workspace.OpModify:
				// Patch with -R handles it
			}
		}

		switch op {
		case workspace.OpCreate:
			os.WriteFile(absPath, []byte(fc.Content), 0644)
		case workspace.OpMkdir:
			os.MkdirAll(absPath, 0755)
		case workspace.OpRmdir:
			os.RemoveAll(absPath)
		case workspace.OpDelete:
			os.Remove(absPath)
		case workspace.OpMove:
			absNew, _ := e.SafeJoin(fc.Content)
			os.Rename(absPath, absNew)
		case workspace.OpModify:
			// Use patch
			args := []string{}
			if reverse {
				args = append(args, "-R")
			}
			args = append(args, absPath)

			cmd := exec.Command("patch", args...)
			stdin, _ := cmd.StdinPipe()
			go func() {
				defer stdin.Close()
				io.WriteString(stdin, fc.Content)
			}()

			if output, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("patch error: %v, output: %s", err, string(output))
			}
		}
	}
	return nil
}
