package utilities

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// Settings はアプリケーションの設定を保持する構造体
type Settings struct {
	LogLevel             string                     `json:"log_level"`
	RegularTask          RegularTaskConfig          `json:"regular_task"`
	RegisteredContainers map[string]ContainerConfig `json:"registered_containers"`
	MessageDeleteAfter   int                        `json:"message_deleteafter"`
	AllowedActions       AllowedActions             `json:"allowed_actions"`
	Icons                map[string]string          `json:"icons"`
}

// RegularTaskConfig は定期タスクの設定
type RegularTaskConfig struct {
	Interval          int `json:"interval"`            // 秒
	AutoShutdownDelay int `json:"auto_shutdown_delay"` // 秒
}

// ContainerConfig は各コンテナの設定
type ContainerConfig struct {
	DisplayName   string `json:"display_name"`
	ContainerName string `json:"container_name"`
	Path          string `json:"path"`
	Icon          string `json:"icon"`
	AutoShutdown  bool   `json:"auto_shutdown"`
}

// AllowedActions は許可するアクション
type AllowedActions struct {
	PowerOn      bool `json:"power_on"`
	PowerOff     bool `json:"power_off"`
	Terminate    bool `json:"terminate"`
	ShowStatus   bool `json:"show_status"`
	PlaceButtons bool `json:"place_buttons"`
}

// LoadSettings は設定ファイルを読み込む
func LoadSettings(path string) (*Settings, error) {
	if path == "" {
		path = os.Getenv("SETTINGS_PATH")
		if path == "" {
			path = "settings.json"
		}
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open settings file: %w", err)
	}
	defer file.Close()

	// flock でファイルロック（読み取り共有ロック）
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_SH); err != nil {
		return nil, fmt.Errorf("failed to lock settings file: %w", err)
	}
	defer syscall.Flock(int(file.Fd()), syscall.LOCK_UN)

	var settings Settings
	if err := json.NewDecoder(file).Decode(&settings); err != nil {
		return nil, fmt.Errorf("failed to parse settings JSON: %w", err)
	}

	// バリデーション
	if err := settings.Validate(); err != nil {
		return nil, fmt.Errorf("invalid settings: %w", err)
	}

	return &settings, nil
}

// SaveSettings は設定を atomic に書き込む
func SaveSettings(path string, settings *Settings) error {
	if path == "" {
		path = os.Getenv("SETTINGS_PATH")
		if path == "" {
			path = "settings.json"
		}
	}

	// バリデーション
	if err := settings.Validate(); err != nil {
		return fmt.Errorf("invalid settings: %w", err)
	}

	// JSON にマーシャル
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	// atomic write: 一時ファイル → rename
	dir := filepath.Dir(path)
	tmpFile, err := os.CreateTemp(dir, "settings-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath) // 失敗時のクリーンアップ

	// flock で排他ロック
	if err := syscall.Flock(int(tmpFile.Fd()), syscall.LOCK_EX); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to lock temp file: %w", err)
	}

	// 書き込み
	if _, err := tmpFile.Write(data); err != nil {
		syscall.Flock(int(tmpFile.Fd()), syscall.LOCK_UN)
		tmpFile.Close()
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// fsync で確実にディスクに書き込み
	if err := tmpFile.Sync(); err != nil {
		syscall.Flock(int(tmpFile.Fd()), syscall.LOCK_UN)
		tmpFile.Close()
		return fmt.Errorf("failed to sync temp file: %w", err)
	}

	// ロック解除とファイルクローズ
	syscall.Flock(int(tmpFile.Fd()), syscall.LOCK_UN)
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// atomic rename
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// Validate は設定の妥当性をチェック
func (s *Settings) Validate() error {
	if s.RegularTask.Interval <= 0 {
		return fmt.Errorf("regular_task.interval must be > 0, got %d", s.RegularTask.Interval)
	}
	if s.RegularTask.AutoShutdownDelay < 0 {
		return fmt.Errorf("regular_task.auto_shutdown_delay must be >= 0, got %d", s.RegularTask.AutoShutdownDelay)
	}
	if len(s.RegisteredContainers) == 0 {
		return fmt.Errorf("registered_containers must not be empty")
	}
	for key, c := range s.RegisteredContainers {
		if c.ContainerName == "" {
			return fmt.Errorf("container %s: container_name is required", key)
		}
		if c.DisplayName == "" {
			return fmt.Errorf("container %s: display_name is required", key)
		}
	}
	return nil
}
