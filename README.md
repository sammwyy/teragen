# ✦ TERAGEN

A terminal-based AI programming agent written in Go. Teragen is designed for speed, visual excellence, and a seamless developer experience.

## ✨ Features

- **Multi-Agent Management**: Seamlessly handle multiple AI sessions with dynamic tabs and sequential ID management.
- **Premium TUI Experience**: A modern, aesthetically pleasing interface built with BubbleTea, LipGloss, and Glamour.
- **Snapshot System**: Automatic tracking of every file change. Teragen records "before" and "after" states to ensure you never lose work.
- **Undo & Redo**: Intelligent rollback and re-apply capabilities for snapshots, allowing you to iterate safely.
- **Secure Sandboxed Access**: Agent tools are restricted to the workspace root with built-in path traversal protection.
- **Visual Diff Summaries**: Real-time ASCII-connected summaries of file operations (Created, Modified, Deleted, Moved).
- **Intelligent Header**: Context-aware path display with smart trimming and terminal width awareness.
- **Provider-Agnostic**: Full support for OpenAI, OpenRouter, and custom OpenAI-compatible endpoints.
- **Secure Secrets**: API tokens and sensitive data are encrypted using hardware-locked IDs (HWID).
- **Local Chat Persistence**: Works like Git, storing sessions in a `.teragen` directory within your project.

## 🍒 Supported providers

- [OpenAI](https://openai.com/)
- [OpenRouter](https://openrouter.ai/)
- Custom OpenAI-compatible endpoints

## 🚀 Quick Start

### Installation
Ensure you have Go 1.21+ installed on your system.

```bash
go build -o teragen ./cmd/teragen
```

### Running
Start the interface directly from your project root:

```bash
./teragen
```

## 🛠 Shortcuts & Navigation

| Shortcut | Action |
| :------- | :----- |
| `SHIFT + →` | Switch to next agent (or Create if at the end) |
| `SHIFT + ←` | Switch to previous agent (Circular) |
| `SHIFT + ↑` | Scroll viewport up |
| `SHIFT + ↓` | Scroll viewport down |
| `UP` / `DOWN` | Navigate through command history |
| `ESC` | Exit the application |
| `ENTER` | Send message / Create agent (when on `[+]` tab) |

## ⌨️ Common Commands

- `/model` — Set model for active provider: `/model <name>`
- `/clear` — Clear visual chat history
- `/chat` — Manage chat: `/chat <clear|new|switch|delete|copy> [args]`
- `/settings` — Manage settings: `/settings <key> <value>`
- `/undo` — Revert the last file changes made by the agent
- `/redo` — Redo the next file changes
- `/provider` — Manage providers: `/provider <list|toggle|remove|set-token> [args]`
- `/help` — Show available commands

## 🏗 Setup & Configuration
Teragen stores its global state and encrypted secrets in your OS standard directories:
- **Windows**: `%APPDATA%\teragen`
- **Linux/macOS**: `~/.config/teragen`

Project-specific chats and local sessions are kept in a `.teragen` folder at your project root.

## � Acknowledgements

Teragen is built on top of amazing open-source projects:

- **[BubbleTea](https://github.com/charmbracelet/bubbletea)**: The TUI framework for Go.
- **[LipGloss](https://github.com/charmbracelet/lipgloss)**: Styling for terminal applications.
- **[Glamour](https://github.com/charmbracelet/glamour)**: Markdown rendering for the CLI.
- **[MachineID](https://github.com/denisbrodbeck/machineid)**: Secure hardware identification.

## �📄 License
MIT © [Sammwyy](https://github.com/sammwy)
