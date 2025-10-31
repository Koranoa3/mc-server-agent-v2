package container

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

// Player はマインクラフトのプレイヤー情報
type Player struct {
	Name string
	UUID string
}

// Container はコンテナの情報と操作
type Container struct {
	ID          string
	Name        string
	Status      WorkingStatus
	Image       string
	Health      string
	Players     []Player
	LastChecked time.Time
	StateHash   string

	client *client.Client
}

// NewContainer は新しい Container を作成
func NewContainer(cli *client.Client, id, name string) *Container {
	return &Container{
		ID:      id,
		Name:    name,
		Status:  StatusUnknown,
		Players: []Player{},
		client:  cli,
	}
}

// Update はコンテナの最新情報を取得して更新
func (c *Container) Update(ctx context.Context) error {
	inspect, err := c.client.ContainerInspect(ctx, c.ID)
	if err != nil {
		c.Status = StatusNotFound
		return fmt.Errorf("failed to inspect container: %w", err)
	}

	c.Image = inspect.Config.Image
	c.LastChecked = time.Now()

	// 稼働状態の判定
	if inspect.State.Running {
		// ヘルスチェックがある場合
		if inspect.State.Health != nil {
			c.Health = inspect.State.Health.Status
			switch inspect.State.Health.Status {
			case "healthy":
				c.Status = StatusRunning
			case "starting":
				c.Status = StatusStarting
			default:
				c.Status = StatusRunning // unhealthy でも running 扱い
			}
		} else {
			c.Status = StatusRunning
		}
	} else {
		c.Status = StatusStopped
	}

	// ハッシュ生成（状態変更検知用）
	c.StateHash = c.computeHash()

	return nil
}

// Start はコンテナを起動
func (c *Container) Start(ctx context.Context) error {
	if err := c.client.ContainerStart(ctx, c.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}
	return c.Update(ctx)
}

// Stop はコンテナを停止
func (c *Container) Stop(ctx context.Context, timeout int) error {
	stopTimeout := timeout
	if err := c.client.ContainerStop(ctx, c.ID, container.StopOptions{Timeout: &stopTimeout}); err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}
	return c.Update(ctx)
}

// Restart はコンテナを再起動
func (c *Container) Restart(ctx context.Context, timeout int) error {
	stopTimeout := timeout
	if err := c.client.ContainerRestart(ctx, c.ID, container.StopOptions{Timeout: &stopTimeout}); err != nil {
		return fmt.Errorf("failed to restart container: %w", err)
	}
	return c.Update(ctx)
}

// computeHash は現在の状態からハッシュを計算
func (c *Container) computeHash() string {
	data := fmt.Sprintf("%s-%s-%s-%d", c.ID, c.Status.String(), c.Health, len(c.Players))
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash[:8]) // 最初の8バイトのみ
}

// HasChanged は前回から状態が変わったかチェック
func (c *Container) HasChanged(previousHash string) bool {
	return c.StateHash != previousHash
}
