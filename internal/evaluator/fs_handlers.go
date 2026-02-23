package evaluator

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sammwy/teragen/internal/workspace"
)

// ValidateFilename ensures the filename is valid for the OS.
func ValidateFilename(name string) error {
	// Basic Unix/Windows safe check
	invalid := regexp.MustCompile(`[<>:"|?*]`)
	if invalid.MatchString(name) {
		return fmt.Errorf("invalid characters in filename: %s", name)
	}
	return nil
}

func (e *Evaluator) registerFSTools() {
	// fs:read
	e.ToolRegistry.Register("fs:read", "Read a file with optional line range", map[string]any{
		"type": "object",
		"properties": map[string]any{
			"target":     map[string]any{"type": "string", "description": "File path"},
			"first":      map[string]any{"type": "integer", "description": "Lines from start"},
			"last":       map[string]any{"type": "integer", "description": "Lines from end"},
			"rangeStart": map[string]any{"type": "integer", "description": "Range start line"},
			"rangeEnd":   map[string]any{"type": "integer", "description": "Range end line"},
		},
		"required": []string{"target"},
	}, func(agentID string, args map[string]any) (string, error) {
		target, _ := args["target"].(string)
		path, err := e.Workspace.SafeJoin(target)
		if err != nil {
			return "", err
		}

		data, err := e.Workspace.ReadFile(path)
		if err != nil {
			return "", err
		}

		lines := strings.Split(string(data), "\n")
		total := len(lines)

		start, end := 0, total

		if v, ok := args["first"].(float64); ok {
			end = int(v)
		} else if v, ok := args["last"].(float64); ok {
			start = total - int(v)
		} else if v1, ok1 := args["rangeStart"].(float64); ok1 {
			start = int(v1) - 1
			if v2, ok2 := args["rangeEnd"].(float64); ok2 {
				end = int(v2)
			}
		}

		if start < 0 {
			start = 0
		}
		if end > total {
			end = total
		}
		if start > end {
			start = end
		}

		return strings.Join(lines[start:end], "\n"), nil
	})

	// fs:write
	e.ToolRegistry.Register("fs:write", "Write or overwrite a file", map[string]any{
		"type": "object",
		"properties": map[string]any{
			"file":    map[string]any{"type": "string", "description": "File path"},
			"content": map[string]any{"type": "string", "description": "File content"},
		},
		"required": []string{"file", "content"},
	}, func(agentID string, args map[string]any) (string, error) {
		file, _ := args["file"].(string)
		content, _ := args["content"].(string)

		path, err := e.Workspace.SafeJoin(file)
		if err != nil {
			return "", err
		}

		if err := e.Workspace.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return "", err
		}

		e.RecordFile(path)

		// Determine create vs modify (best-effort)
		op := workspace.OpCreate
		if _, err := e.Workspace.ReadFile(path); err == nil {
			op = workspace.OpModify
		}
		if e.ActiveSnapshot != nil {
			e.ActiveSnapshot.RecordOp(path, op)
		}

		err = e.Workspace.WriteFile(path, []byte(content))
		if err != nil {
			return "", err
		}
		return "File written successfully", nil
	})

	// fs:ls
	e.ToolRegistry.Register("fs:ls", "List files in a directory", map[string]any{
		"type": "object",
		"properties": map[string]any{
			"target": map[string]any{"type": "string", "description": "Directory path", "default": "."},
		},
	}, func(agentID string, args map[string]any) (string, error) {
		target := "."
		if v, ok := args["target"].(string); ok {
			target = v
		}

		path, err := e.Workspace.SafeJoin(target)
		if err != nil {
			return "", err
		}

		entries, err := e.Workspace.ReadDir(path)
		if err != nil {
			return "", err
		}

		var out []string
		for _, entry := range entries {
			typeStr := "F"
			if entry.IsDir {
				typeStr = "D"
			}
			out = append(out, fmt.Sprintf("[%s] %s", typeStr, entry.Name))
		}
		return strings.Join(out, "\n"), nil
	})

	// fs:mkdir
	e.ToolRegistry.Register("fs:mkdir", "Create a directory", map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{"type": "string", "description": "Directory name"},
		},
		"required": []string{"name"},
	}, func(agentID string, args map[string]any) (string, error) {
		name, _ := args["name"].(string)
		path, err := e.Workspace.SafeJoin(name)
		if err != nil {
			return "", err
		}
		if e.ActiveSnapshot != nil {
			e.ActiveSnapshot.RecordOp(path, workspace.OpMkdir)
		}
		err = e.Workspace.MkdirAll(path, 0o755)
		if err != nil {
			return "", err
		}
		return "Directory created", nil
	})

	// fs:modify (Simplified version: overwrite with new content for now, or use patch)
	e.ToolRegistry.Register("fs:modify", "Modify a file using a diff or new content", map[string]any{
		"type": "object",
		"properties": map[string]any{
			"file":    map[string]any{"type": "string", "description": "File path"},
			"content": map[string]any{"type": "string", "description": "New content (or diff)"},
		},
		"required": []string{"file", "content"},
	}, func(agentID string, args map[string]any) (string, error) {
		file, _ := args["file"].(string)
		content, _ := args["content"].(string)
		path, err := e.Workspace.SafeJoin(file)
		if err != nil {
			return "", err
		}
		if err := e.Workspace.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return "", err
		}
		e.RecordFile(path)
		if e.ActiveSnapshot != nil {
			e.ActiveSnapshot.RecordOp(path, workspace.OpModify)
		}
		err = e.Workspace.WriteFile(path, []byte(content))
		return "File modified", err
	})

	// fs:unlink
	e.ToolRegistry.Register("fs:unlink", "Delete a file", map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{"type": "string", "description": "File name"},
		},
		"required": []string{"name"},
	}, func(agentID string, args map[string]any) (string, error) {
		name, _ := args["name"].(string)
		path, err := e.Workspace.SafeJoin(name)
		if err != nil {
			return "", err
		}
		e.RecordFile(path)
		if e.ActiveSnapshot != nil {
			e.ActiveSnapshot.RecordOp(path, workspace.OpDelete)
		}
		err = e.Workspace.Remove(path)
		return "File deleted", err
	})

	// fs:rmdir
	e.ToolRegistry.Register("fs:rmdir", "Delete a directory", map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{"type": "string", "description": "Directory name"},
		},
		"required": []string{"name"},
	}, func(agentID string, args map[string]any) (string, error) {
		name, _ := args["name"].(string)
		path, err := e.Workspace.SafeJoin(name)
		if err != nil {
			return "", err
		}
		if e.ActiveSnapshot != nil {
			e.ActiveSnapshot.RecordOp(path, workspace.OpRmdir)
		}
		err = e.Workspace.RemoveAll(path)
		return "Directory removed", err
	})

	// fs:move
	e.ToolRegistry.Register("fs:move", "Move or rename a file/directory", map[string]any{
		"type": "object",
		"properties": map[string]any{
			"old": map[string]any{"type": "string", "description": "Old path"},
			"new": map[string]any{"type": "string", "description": "New path"},
		},
		"required": []string{"old", "new"},
	}, func(agentID string, args map[string]any) (string, error) {
		oldPath, _ := args["old"].(string)
		newPath, _ := args["new"].(string)
		src, err := e.Workspace.SafeJoin(oldPath)
		if err != nil {
			return "", err
		}
		dst, err := e.Workspace.SafeJoin(newPath)
		if err != nil {
			return "", err
		}
		if e.ActiveSnapshot != nil {
			e.ActiveSnapshot.RecordOp(src, workspace.OpMove, dst)
		}
		err = e.Workspace.Rename(src, dst)
		return "Moved successfully", err
	})

	// fs:find
	e.ToolRegistry.Register("fs:find", "Search for files using regex", map[string]any{
		"type": "object",
		"properties": map[string]any{
			"expr": map[string]any{"type": "string", "description": "Regex pattern"},
		},
		"required": []string{"expr"},
	}, func(agentID string, args map[string]any) (string, error) {
		expr, _ := args["expr"].(string)
		re, err := regexp.Compile(expr)
		if err != nil {
			return "", err
		}

		var matches []string
		err = e.Workspace.Walk(".", func(rel string, isDir bool) error {
			_ = isDir
			if re.MatchString(rel) {
				matches = append(matches, rel)
			}
			return nil
		})

		if len(matches) == 0 {
			return "No files found", nil
		}
		return strings.Join(matches, "\n"), nil
	})
}
