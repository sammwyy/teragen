package evaluator

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/pmezard/go-difflib/difflib"
	"github.com/sammwy/teragen/internal/workspace"
)

type SnapshotSession struct {
	ID        string
	ChatID    string
	StartTime time.Time
	Originals map[string]originalFile         // AbsPath -> original
	Changes   map[string]workspace.FileChange // Path -> Operation Info
	Workspace workspace.Workspace
}

type originalFile struct {
	Exists  bool
	Content string
}

func (e *Evaluator) StartSnapshotSession(chatID string) *SnapshotSession {
	return &SnapshotSession{
		ID:        uuid.New().String(),
		ChatID:    chatID,
		StartTime: time.Now(),
		Originals: make(map[string]originalFile),
		Changes:   make(map[string]workspace.FileChange),
		Workspace: e.Workspace,
	}
}

func (s *SnapshotSession) RecordFile(path string) error {
	if _, ok := s.Originals[path]; ok {
		return nil
	}

	data, err := s.Workspace.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			s.Originals[path] = originalFile{Exists: false, Content: ""}
			return nil
		}
		return err
	}

	s.Originals[path] = originalFile{Exists: true, Content: string(data)}
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

	absRoot := s.Workspace.Root()

	// Process all files that were modified or might have been changed
	for originalPath, orig := range s.Originals {
		rel, _ := filepath.Rel(absRoot, originalPath)
		change, hasExplicitOp := s.Changes[originalPath]

		op := workspace.OpModify

		if hasExplicitOp {
			op = change.Operation
		} else if !orig.Exists {
			op = workspace.OpCreate
		}

		fc := workspace.FileChange{
			Path:      filepath.ToSlash(rel),
			Operation: op,
		}

		switch op {
		case workspace.OpMove:
			relNew, _ := filepath.Rel(absRoot, change.Content)
			fc.Content = filepath.ToSlash(relNew)

		case workspace.OpDelete:
			// content before deletion
			fc.Before = orig.Content

		case workspace.OpCreate, workspace.OpModify:
			afterData, err := s.Workspace.ReadFile(originalPath)
			after := ""
			if err == nil {
				after = string(afterData)
			}

			before := ""
			if orig.Exists {
				before = orig.Content
			}

			fc.Before = before
			fc.After = after

			if op == workspace.OpCreate {
				// Keep "Content" as full file content for rendering stats.
				fc.Content = after
			} else {
				ud := difflib.UnifiedDiff{
					A:        difflib.SplitLines(before),
					B:        difflib.SplitLines(after),
					FromFile: "a/" + filepath.ToSlash(rel),
					ToFile:   "b/" + filepath.ToSlash(rel),
					Context:  3,
				}
				diffText, _ := difflib.GetUnifiedDiffString(ud)
				fc.Content = diffText
			}
		}

		finalChanges[fc.Path] = fc
	}

	// Process explicit operations that didn't have an original file (like MKDIR or CREATE on non-existing path)
	for path, change := range s.Changes {
		rel, _ := filepath.Rel(absRoot, path)
		rel = filepath.ToSlash(rel)
		if _, ok := finalChanges[rel]; ok {
			continue
		}

		op := change.Operation
		fc := workspace.FileChange{Path: rel, Operation: op}
		switch op {
		case workspace.OpMove:
			relNew, _ := filepath.Rel(absRoot, change.Content)
			fc.Content = filepath.ToSlash(relNew)
		case workspace.OpCreate, workspace.OpModify:
			afterData, err := s.Workspace.ReadFile(path)
			if err == nil {
				fc.After = string(afterData)
				fc.Content = fc.After
			}
		}
		finalChanges[rel] = fc
	}

	for _, fc := range finalChanges {
		snap.Files = append(snap.Files, fc)
	}

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
		absPath, err := e.Workspace.SafeJoin(fc.Path)
		if err != nil {
			return err
		}

		op := fc.Operation
		switch op {
		case workspace.OpCreate:
			if reverse {
				_ = e.Workspace.Remove(absPath)
				continue
			}
			if err := e.Workspace.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
				return err
			}
			content := fc.After
			if content == "" {
				content = fc.Content
			}
			if err := e.Workspace.WriteFile(absPath, []byte(content)); err != nil {
				return err
			}
		case workspace.OpMkdir:
			if reverse {
				_ = e.Workspace.RemoveAll(absPath)
				continue
			}
			if err := e.Workspace.MkdirAll(absPath, 0o755); err != nil {
				return err
			}
		case workspace.OpRmdir:
			if reverse {
				if err := e.Workspace.MkdirAll(absPath, 0o755); err != nil {
					return err
				}
				continue
			}
			if err := e.Workspace.RemoveAll(absPath); err != nil {
				return err
			}
		case workspace.OpDelete:
			if reverse {
				if fc.Before == "" {
					continue
				}
				if err := e.Workspace.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
					return err
				}
				if err := e.Workspace.WriteFile(absPath, []byte(fc.Before)); err != nil {
					return err
				}
				continue
			}
			_ = e.Workspace.Remove(absPath)
		case workspace.OpMove:
			absNew, _ := e.Workspace.SafeJoin(fc.Content)
			if reverse {
				_ = e.Workspace.Rename(absNew, absPath)
				continue
			}
			_ = e.Workspace.Rename(absPath, absNew)
		case workspace.OpModify:
			if err := e.Workspace.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
				return err
			}
			content := fc.After
			if reverse {
				content = fc.Before
			}
			if err := e.Workspace.WriteFile(absPath, []byte(content)); err != nil {
				return fmt.Errorf("modify error: %v", err)
			}
		}
	}
	return nil
}
