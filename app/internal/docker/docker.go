package docker

import (
	"context"
	"fmt"

	"github.com/Koranoa3/mc-server-agent/internal/docker/container"
	"github.com/Koranoa3/mc-server-agent/internal/state"
	dockertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

// Manager は Docker コンテナを管理
type Manager struct {
	client *client.Client
	state  *state.AppState
}

// NewManager は新しい Manager を作成
func NewManager(state *state.AppState) (*Manager, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	return &Manager{
		client: cli,
		state:  state,
	}, nil
}

// Close はDocker clientをクローズ
func (m *Manager) Close() error {
	if m.client != nil {
		return m.client.Close()
	}
	return nil
}

// UpdateAllContainers は設定に登録された全コンテナの情報を更新
func (m *Manager) UpdateAllContainers(ctx context.Context) error {
	settings := m.state.GetSettings()

	for key, cfg := range settings.RegisteredContainers {
		// コンテナ名で検索
		containers, err := m.client.ContainerList(ctx, dockertypes.ListOptions{All: true})
		if err != nil {
			return fmt.Errorf("failed to list containers: %w", err)
		}

		var found bool
		for _, c := range containers {
			// コンテナ名が一致するか確認（名前は "/name" 形式なので注意）
			for _, name := range c.Names {
				if name == "/"+cfg.ContainerName || name == cfg.ContainerName {
					// コンテナを作成または更新
					cont := container.NewContainer(m.client, c.ID, cfg.ContainerName)
					if err := cont.Update(ctx); err != nil {
						return fmt.Errorf("failed to update container %s: %w", key, err)
					}
					m.state.UpdateContainer(key, cont)
					found = true
					break
				}
			}
			if found {
				break
			}
		}

		// 存在しない場合も state に記録（StatusNotFound）
		if !found {
			cont := container.NewContainer(m.client, "", cfg.ContainerName)
			cont.Status = container.StatusNotFound
			m.state.UpdateContainer(key, cont)
		}
	}

	return nil
}

// StartContainer はコンテナを起動
func (m *Manager) StartContainer(ctx context.Context, key string) error {
	settings := m.state.GetSettings()
	_, ok := settings.RegisteredContainers[key]
	if !ok {
		return fmt.Errorf("container %s not found in settings", key)
	}

	// 現在の state からコンテナ取得
	stateContainer, ok := m.state.GetContainer(key)
	if !ok {
		return fmt.Errorf("container %s not found", key)
	}

	cont, ok := stateContainer.(*container.Container)
	if !ok || cont.ID == "" {
		return fmt.Errorf("container %s ID unknown", key)
	}

	if err := cont.Start(ctx); err != nil {
		return err
	}

	m.state.UpdateContainer(key, cont)
	return nil
}

// StopContainer はコンテナを停止
func (m *Manager) StopContainer(ctx context.Context, key string, timeout int) error {
	settings := m.state.GetSettings()
	_, ok := settings.RegisteredContainers[key]
	if !ok {
		return fmt.Errorf("container %s not found in settings", key)
	}

	stateContainer, ok := m.state.GetContainer(key)
	if !ok {
		return fmt.Errorf("container %s not found", key)
	}

	cont, ok := stateContainer.(*container.Container)
	if !ok || cont.ID == "" {
		return fmt.Errorf("container %s ID unknown", key)
	}

	if err := cont.Stop(ctx, timeout); err != nil {
		return err
	}

	m.state.UpdateContainer(key, cont)
	return nil
}

// RestartContainer はコンテナを再起動
func (m *Manager) RestartContainer(ctx context.Context, key string, timeout int) error {
	settings := m.state.GetSettings()
	_, ok := settings.RegisteredContainers[key]
	if !ok {
		return fmt.Errorf("container %s not found in settings", key)
	}

	stateContainer, ok := m.state.GetContainer(key)
	if !ok {
		return fmt.Errorf("container %s not found", key)
	}

	cont, ok := stateContainer.(*container.Container)
	if !ok || cont.ID == "" {
		return fmt.Errorf("container %s ID unknown", key)
	}

	if err := cont.Restart(ctx, timeout); err != nil {
		return err
	}

	m.state.UpdateContainer(key, cont)
	return nil
}
