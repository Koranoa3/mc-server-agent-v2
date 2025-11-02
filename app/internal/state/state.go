package state

import (
	"sync"

	"github.com/Koranoa3/mc-server-agent/internal/utilities"
)

// Container は docker/container.Container へのエイリアス
// NOTE: 循環import回避のため、ここでは interface{} で保持し、
// 実際の型は docker パッケージで扱う
type Container interface{}

// AppState はアプリケーション全体の状態を管理
type AppState struct {
	mu         sync.RWMutex
	settings   *utilities.Settings
	containers map[string]Container
}

// NewAppState は新しい AppState を作成
func NewAppState(settings *utilities.Settings) *AppState {
	return &AppState{
		settings:   settings,
		containers: make(map[string]Container),
	}
}

// GetSettings は設定を取得（読み取り専用）
func (s *AppState) GetSettings() *utilities.Settings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.settings
}

// UpdateSettings は設定を更新
func (s *AppState) UpdateSettings(settings *utilities.Settings) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.settings = settings
}

// GetContainer は指定IDのコンテナを取得
func (s *AppState) GetContainer(id string) (Container, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.containers[id]
	return c, ok
}

// GetAllContainers は全コンテナを取得
func (s *AppState) GetAllContainers() map[string]Container {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// コピーを返す（外部での変更を防ぐ）
	result := make(map[string]Container, len(s.containers))
	for k, v := range s.containers {
		result[k] = v
	}
	return result
}

// UpdateContainer はコンテナ情報を更新
func (s *AppState) UpdateContainer(id string, container Container) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.containers[id] = container
}

// DeleteContainer はコンテナを削除
func (s *AppState) DeleteContainer(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.containers, id)
}

// ClearContainers は全コンテナをクリア
func (s *AppState) ClearContainers() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.containers = make(map[string]Container)
}
