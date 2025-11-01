package container

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"regexp"
	"strings"
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
	StopTimer   time.Time
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

// SetClient sets the docker client for the container (exported so callers from other packages can set it)
func (c *Container) SetClient(cli *client.Client) {
	c.client = cli
}

// SetID sets the container ID
func (c *Container) SetID(id string) {
	c.ID = id
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
				c.StopTimer = time.Now()
			default:
				c.Status = StatusRunning // unhealthy でも running 扱い
			}
		} else {
			c.Status = StatusRunning
		}

		// 仮実装：プレイヤー情報を取得（後で実装予定の RCON/exec に置き換える）
		players, perr := c.fetchPlayers(ctx)
		if perr != nil {
			// プレイヤー取得失敗は致命的にしない。ログは呼び出し側で行う想定。
		} else {
			c.Players = players
			// プレイヤーが存在する場合は StopTimer を更新
			if len(players) > 0 {
				c.StopTimer = time.Now()
			}
		}

	} else {
		// 停止中はプレイヤーリストをクリア
		c.Status = StatusStopped
		c.Players = []Player{}
	}

	// ハッシュ生成（状態変更検知用）
	c.StateHash = c.computeHash()

	return nil
}

// fetchPlayers はプレイヤー一覧を取得する仮実装
func (c *Container) fetchPlayers(ctx context.Context) ([]Player, error) {
	// rcon-cli list コマンドを実行
	execConfig := container.ExecOptions{
		Cmd:          []string{"rcon-cli", "list"},
		AttachStdout: true,
		AttachStderr: true,
	}

	execID, err := c.client.ContainerExecCreate(ctx, c.ID, execConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create exec: %w", err)
	}

	resp, err := c.client.ContainerExecAttach(ctx, execID.ID, container.ExecStartOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to attach exec: %w", err)
	}
	defer resp.Close()

	// 出力を読み取る
	output, err := io.ReadAll(resp.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read exec output: %w", err)
	}

	// "players online: (.+)" の正規表現でマッチング
	re := regexp.MustCompile(`players online:\s*(.+)`)
	matches := re.FindStringSubmatch(string(output))
	if len(matches) < 2 {
		// マッチしない場合はプレイヤーなし
		return []Player{}, nil
	}

	playerNames := matches[1]
	if playerNames == "" || playerNames == "0" {
		return []Player{}, nil
	}

	// カンマ区切りでプレイヤー名を分割
	names := strings.Split(playerNames, " ,")
	players := make([]Player, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name != "" {
			players = append(players, Player{
				Name: name,
				UUID: "", // RCON では UUID が取得できないため空
			})
		}
	}

	return players, nil
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
