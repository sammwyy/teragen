# ✦ TERAGEN

A terminal-based AI programming agent written in Go. Teragen is designed for speed, visual excellence, and a seamless developer experience.

## ✨ Features

- **Multi-Agent Management**: Seamlessly handle multiple AI sessions with dynamic tabs and sequential ID management.
- **Premium TUI Experience**: A modern, aesthetically pleasing interface built with BubbleTea, LipGloss, and Glamour.
- **Intelligent Header**: Context-aware path display with smart trimming and terminal width awareness.
- **Provider-Agnostic**: Full support for OpenAI, OpenRouter, and custom OpenAI-compatible endpoints.
- **Secure Secrets**: API tokens and sensitive data are encrypted using hardware-locked IDs (HWID) to prevent leakage.
- **Smart Metadata**: Real-time token counting per message, session-wide usage tracking, and intelligent relative timestamps.
- **Workspace-Aware**: Local chat persistence and configuration managed within the `.teragen` directory of your project.

## 🚀 Quick Start

### Installation
Ensure you have Go 1.21+ installed on your system.

```powershell
go build -o teragen ./cmd/teragen
```

### Running
Start the interface directly from your project root:

```powershell
./teragen
```

## 🛠 Shortcuts & Navigation

| Shortcut | Action |
| :------- | :----- |
| `SHIFT + →` | Switch to next agent (or Create if at the end) |
| `SHIFT + ←` | Switch to previous agent (Circular) |
| `UP` / `DOWN` | Navigate through command history |
| `ESC` | Exit the application |
| `ENTER` | Send message / Create agent (when on `[+]` tab) |

## ⌨️ Common Commands

- `/provider list`: List all registered AI providers.
- `/provider toggle <id>`: Switch the active provider.
- `/clear`: Clear the current chat history.
- `/help`: Display the available terminal commands.

## 📂 Architecture

- `internal/agent`: Agent orchestration, lifecycle, and history management.
- `internal/ai`: AI implementation layer (OpenAI, OpenRouter providers).
- `internal/core`: Central event bus and agent registration.
- `internal/processor`: Command handling and input processing.
- `internal/ui`: The heart of the TUI, rendering the interface and handling events.
- `internal/workspace`: Disk persistence for chats (`_chats/`) and agent states.

## 🏗 Setup & Configuration
Teragen stores its global state and encrypted secrets in your OS standard directories:
- **Windows**: `%APPDATA%\teragen`
- **Linux/macOS**: `~/.config/teragen`

Project-specific chats and local sessions are kept in a `.teragen` folder at your project root.

## 📄 License
MIT © [Sammwyy](https://github.com/sammwyy)
