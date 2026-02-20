package evaluator

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// SafeJoin joins the workspace root with a path and ensures no path traversal.
func (e *Evaluator) SafeJoin(target string) (string, error) {
	absRoot, err := filepath.Abs(e.Workspace.Root)
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
		path, err := e.SafeJoin(target)
		if err != nil {
			return "", err
		}

		data, err := os.ReadFile(path)
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

		path, err := e.SafeJoin(file)
		if err != nil {
			return "", err
		}

		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return "", err
		}

		e.RecordFile(path)
		err = os.WriteFile(path, []byte(content), 0644)
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

		path, err := e.SafeJoin(target)
		if err != nil {
			return "", err
		}

		entries, err := os.ReadDir(path)
		if err != nil {
			return "", err
		}

		var out []string
		for _, entry := range entries {
			typeStr := "F"
			if entry.IsDir() {
				typeStr = "D"
			}
			out = append(out, fmt.Sprintf("[%s] %s", typeStr, entry.Name()))
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
		path, err := e.SafeJoin(name)
		if err != nil {
			return "", err
		}
		err = os.MkdirAll(path, 0755)
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
		path, err := e.SafeJoin(file)
		if err != nil {
			return "", err
		}
		e.RecordFile(path)
		err = os.WriteFile(path, []byte(content), 0644)
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
		path, err := e.SafeJoin(name)
		if err != nil {
			return "", err
		}
		e.RecordFile(path)
		err = os.Remove(path)
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
		path, err := e.SafeJoin(name)
		if err != nil {
			return "", err
		}
		err = os.RemoveAll(path)
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
		src, err := e.SafeJoin(oldPath)
		if err != nil {
			return "", err
		}
		dst, err := e.SafeJoin(newPath)
		if err != nil {
			return "", err
		}
		err = os.Rename(src, dst)
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
		err = filepath.Walk(e.Workspace.Root, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			rel, _ := filepath.Rel(e.Workspace.Root, p)
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
