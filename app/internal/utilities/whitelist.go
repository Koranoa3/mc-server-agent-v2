package utilities

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"syscall"
	"time"
)

// WhitelistEntry はホワイトリストの1エントリ
type WhitelistEntry struct {
	UUID        string `json:"uuid"`
	Name        string `json:"name"`
	AddedUserID string `json:"added_user_id,omitempty"`
}

// MojangProfile は Mojang API のレスポンス
type MojangProfile struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// FetchMojangProfile はプレイヤー名から UUID を取得
func FetchMojangProfile(playerName string) (*MojangProfile, error) {
	url := fmt.Sprintf("https://api.mojang.com/users/profiles/minecraft/%s", playerName)

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch profile: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("player not found")
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("mojang API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var profile MojangProfile
	if err := json.Unmarshal(body, &profile); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &profile, nil
}

// LoadWhitelist はホワイトリストファイルを読み込む
func LoadWhitelist(path string) ([]WhitelistEntry, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			// ファイルが存在しない場合は空のリストを返す
			return []WhitelistEntry{}, nil
		}
		return nil, fmt.Errorf("failed to open whitelist file: %w", err)
	}
	defer file.Close()

	// 読み取り共有ロック
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_SH); err != nil {
		return nil, fmt.Errorf("failed to lock whitelist file: %w", err)
	}
	defer syscall.Flock(int(file.Fd()), syscall.LOCK_UN)

	var entries []WhitelistEntry
	if err := json.NewDecoder(file).Decode(&entries); err != nil {
		return nil, fmt.Errorf("failed to parse whitelist JSON: %w", err)
	}

	return entries, nil
}

// SaveWhitelist はホワイトリストファイルを保存
func SaveWhitelist(path string, entries []WhitelistEntry) error {
	// JSON にマーシャル
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal whitelist: %w", err)
	}

	// ファイルを開く（存在しなければ作成）
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to open whitelist file: %w", err)
	}
	defer file.Close()

	// 排他ロック
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("failed to lock whitelist file: %w", err)
	}
	defer syscall.Flock(int(file.Fd()), syscall.LOCK_UN)

	// 書き込み
	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("failed to write whitelist file: %w", err)
	}

	// fsync で確実にディスクに書き込み
	if err := file.Sync(); err != nil {
		return fmt.Errorf("failed to sync whitelist file: %w", err)
	}

	return nil
}

// AddToWhitelist はプレイヤーをホワイトリストに追加
func AddToWhitelist(path, uuid, name, addedUserID string) (bool, error) {
	entries, err := LoadWhitelist(path)
	if err != nil {
		return false, err
	}

	// UUIDをハイフン付き形式に変換 (Minecraftの標準形式)
	formattedUUID := formatUUID(uuid)

	// 既存エントリを探す
	found := false
	for i, entry := range entries {
		if entry.UUID == formattedUUID {
			// 既に存在する場合は名前を更新
			entries[i].Name = name
			found = true
			break
		}
	}

	if !found {
		// 新規追加
		entries = append(entries, WhitelistEntry{
			UUID:        formattedUUID,
			Name:        name,
			AddedUserID: addedUserID,
		})
	}

	if err := SaveWhitelist(path, entries); err != nil {
		return false, err
	}

	return !found, nil // true = 新規追加, false = 既に存在
}

// RemoveFromWhitelist はプレイヤーをホワイトリストから削除
func RemoveFromWhitelist(path, uuid string) (bool, error) {
	entries, err := LoadWhitelist(path)
	if err != nil {
		return false, err
	}

	// UUIDをハイフン付き形式に変換
	formattedUUID := formatUUID(uuid)

	// 該当エントリを削除
	found := false
	newEntries := make([]WhitelistEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.UUID == formattedUUID {
			found = true
			continue
		}
		newEntries = append(newEntries, entry)
	}

	if !found {
		return false, nil // 削除するものがなかった
	}

	if err := SaveWhitelist(path, newEntries); err != nil {
		return false, err
	}

	return true, nil // 削除成功
}

// formatUUID はハイフンなしのUUIDをハイフン付き形式に変換
// 例: "069a79f444e94726a5befca90e38aaf5" -> "069a79f4-44e9-4726-a5be-fca90e38aaf5"
func formatUUID(uuid string) string {
	if len(uuid) != 32 {
		return uuid // 既にフォーマット済みまたは無効
	}
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		uuid[0:8],
		uuid[8:12],
		uuid[12:16],
		uuid[16:20],
		uuid[20:32],
	)
}
