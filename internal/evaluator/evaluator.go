package evaluator

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sammwy/teragen/internal/workspace"
)

type Evaluator struct {
	CWD            string
	Workspace      *workspace.Workspace
	ToolRegistry   *ToolRegistry
	ActiveSnapshot *SnapshotSession
}

func NewEvaluator(w *workspace.Workspace) *Evaluator {
	cwd, _ := os.Getwd()
	e := &Evaluator{
		CWD:          cwd,
		Workspace:    w,
		ToolRegistry: NewToolRegistry(),
	}
	e.registerFSTools()
	return e
}

func (e *Evaluator) ExecuteShell(command string) (string, error) {
	// In a real app, this would prompt the user.
	// For this exercise, we'll implement the permission check logic.

	fmt.Printf("\033[33m[Permission Required]\033[0m Execute shell command: %s\n", command)
	fmt.Print("Allow? (y/n): ")
	var input string
	fmt.Scanln(&input)

	if strings.ToLower(input) != "y" {
		return "", fmt.Errorf("permission denied")
	}

	cmd := exec.Command("powershell", "-Command", command)
	if os.PathSeparator == '/' {
		cmd = exec.Command("sh", "-c", command)
	}

	output, err := cmd.CombinedOutput()
	return string(output), err
}

func (e *Evaluator) ReadFile(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	if !strings.HasPrefix(absPath, e.CWD) {
		fmt.Printf("\033[33m[Permission Required]\033[0m Access file outside workspace: %s\n", absPath)
		fmt.Print("Allow? (y/n): ")
		var input string
		fmt.Scanln(&input)
		if strings.ToLower(input) != "y" {
			return "", fmt.Errorf("permission denied")
		}
	}

	data, err := os.ReadFile(absPath)
	return string(data), err
}

func (e *Evaluator) WriteFile(path string, content string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	if !strings.HasPrefix(absPath, e.CWD) {
		fmt.Printf("\033[33m[Permission Required]\033[0m Write file outside workspace: %s\n", absPath)
		fmt.Print("Allow? (y/n): ")
		var input string
		fmt.Scanln(&input)
		if strings.ToLower(input) != "y" {
			return fmt.Errorf("permission denied")
		}
	}

	return os.WriteFile(absPath, []byte(content), 0644)
}

func (e *Evaluator) RecordFile(path string) {
	if e.ActiveSnapshot != nil {
		e.ActiveSnapshot.RecordFile(path)
	}
}
