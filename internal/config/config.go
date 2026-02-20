package config

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/google/uuid"
)

type ProviderConfig struct {
	ID          string  `json:"id"`
	Type        string  `json:"type"`
	DisplayName string  `json:"display_name"`
	Model       string  `json:"model"`
	BaseURL     string  `json:"base_url"`
	MaxTokens   int     `json:"max_tokens"`
	TopP        float64 `json:"top_p"`
	Temperature float64 `json:"temperature"`
}

type ShellConfig struct {
	ID  string `json:"id"`
	Bin string `json:"bin"`
}

type AppConfig struct {
	ActiveProviderID string        `json:"active_provider_id"`
	ActiveShell      string        `json:"active_shell"`
	AvailableShells  []ShellConfig `json:"available_shells"`
}

func GetSystemPromptPath() string {
	return filepath.Join(ConfigDir, "system_prompt.txt")
}

func LoadSystemPrompt(cfg *AppConfig) (string, error) {
	path := GetSystemPromptPath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		defaultPrompt := "You are Teragen (github.com/sammwyy/teragen), a specialized terminal coding agent.\n" +
			"System Information:\n" +
			"- OS: {{os_name}}\n" +
			"- Architecture: {{arch}}\n" +
			"- Default Shell: {{shell}}\n" +
			"\n" +
			"Guidelines:\n" +
			"1. Be concise and professional.\n" +
			"2. Provide ready-to-use terminal commands.\n" +
			"3. Use markdown for better readability."
		os.WriteFile(path, []byte(defaultPrompt), 0644)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	prompt := string(data)
	prompt = strings.ReplaceAll(prompt, "{{os_name}}", runtime.GOOS)
	prompt = strings.ReplaceAll(prompt, "{{arch}}", runtime.GOARCH)
	prompt = strings.ReplaceAll(prompt, "{{shell}}", cfg.ActiveShell)

	return prompt, nil
}

func LoadAppConfig() (*AppConfig, error) {
	var cfg AppConfig
	if _, err := os.Stat(ConfigFile); os.IsNotExist(err) {
		cfg = AppConfig{
			AvailableShells: []ShellConfig{
				{ID: "cmd", Bin: "cmd"},
				{ID: "powershell", Bin: "powershell"},
			},
			ActiveShell: "powershell",
		}
		return &cfg, nil
	}
	data, err := os.ReadFile(ConfigFile)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(data, &cfg)

	// Defaults if missing in existing config
	if len(cfg.AvailableShells) == 0 {
		cfg.AvailableShells = []ShellConfig{
			{ID: "cmd", Bin: "cmd"},
			{ID: "powershell", Bin: "powershell"},
		}
		if cfg.ActiveShell == "" {
			cfg.ActiveShell = "powershell"
		}
	}

	return &cfg, err
}

func SaveAppConfig(cfg *AppConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ConfigFile, data, 0644)
}

func GetProviderDir() string {
	dir := filepath.Join(ConfigDir, "providers")
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		os.MkdirAll(dir, 0755)
	}
	return dir
}

func LoadProviders() ([]ProviderConfig, error) {
	dir := GetProviderDir()
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var providers []ProviderConfig
	for _, f := range files {
		if filepath.Ext(f.Name()) == ".json" {
			data, err := os.ReadFile(filepath.Join(dir, f.Name()))
			if err != nil {
				continue
			}
			var p ProviderConfig
			if err := json.Unmarshal(data, &p); err == nil {
				providers = append(providers, p)
			}
		}
	}
	return providers, nil
}

func SaveProvider(p *ProviderConfig) error {
	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	dir := GetProviderDir()
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, p.ID+".json"), data, 0644)
}

func RemoveProvider(id string) error {
	return os.Remove(filepath.Join(GetProviderDir(), id+".json"))
}

func SetSecret(id string, token string) error {
	hwid, err := GetHWID()
	if err != nil {
		return err
	}
	encrypted, err := encrypt(token, hwid)
	if err != nil {
		return err
	}

	key := fmt.Sprintf("TOKEN_%s", id)
	return updateSecretEnv(key, encrypted)
}

func GetSecret(id string) (string, error) {
	hwid, err := GetHWID()
	if err != nil {
		return "", err
	}
	key := fmt.Sprintf("TOKEN_%s", id)
	encrypted, err := readSecretFromEnv(key)
	if err != nil {
		return "", err
	}
	return decrypt(encrypted, hwid)
}

func updateSecretEnv(key, value string) error {
	secrets, err := loadSecretEnv()
	if err != nil {
		secrets = make(map[string]string)
	}
	secrets[key] = value
	return saveSecretEnv(secrets)
}

func readSecretFromEnv(key string) (string, error) {
	secrets, err := loadSecretEnv()
	if err != nil {
		return "", err
	}
	val, ok := secrets[key]
	if !ok {
		return "", fmt.Errorf("secret not found")
	}
	return val, nil
}

func loadSecretEnv() (map[string]string, error) {
	secrets := make(map[string]string)
	file, err := os.Open(SecretFile)
	if err != nil {
		if os.IsNotExist(err) {
			return secrets, nil
		}
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			secrets[parts[0]] = parts[1]
		}
	}
	return secrets, scanner.Err()
}

func saveSecretEnv(secrets map[string]string) error {
	file, err := os.Create(SecretFile)
	if err != nil {
		return err
	}
	defer file.Close()

	for k, v := range secrets {
		fmt.Fprintf(file, "%s=%s\n", k, v)
	}
	return nil
}
