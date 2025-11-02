package routine

import (
	"context"
	"time"

	"github.com/Koranoa3/mc-server-agent/internal/docker"
	"github.com/Koranoa3/mc-server-agent/internal/docker/container"
	"github.com/Koranoa3/mc-server-agent/internal/state"
	"github.com/rs/zerolog/log"
)

// StatusUpdate は状態変化通知
type StatusUpdate struct {
	ContainerID string
	Changed     bool
}

// Command はコマンド
type Command struct {
	Type        string
	ContainerID string
	Timeout     int
}

// Run は定期監視ループを実行
func Run(ctx context.Context, appState *state.AppState, dockerMgr *docker.Manager, statusChan chan<- StatusUpdate, cmdChan chan<- Command) {
	settings := appState.GetSettings()
	interval := time.Duration(settings.RegularTask.Interval) * time.Second

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Info().Dur("interval", interval).Msg("Routine started")

	// 前回のハッシュを保存
	previousHashes := make(map[string]string)

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Routine shutting down")
			return

		case <-ticker.C:
			log.Debug().Msg("Routine: checking containers")

			// コンテナ情報を更新
			if err := dockerMgr.UpdateAllContainers(ctx); err != nil {
				log.Error().Err(err).Msg("Routine: failed to update containers")
				continue
			}

			// 各コンテナの状態をチェック
			containers := appState.GetAllContainers()
			for key, c := range containers {
				cont, ok := c.(*container.Container)
				if !ok {
					continue
				}

				// 状態変化を検知
				prevHash := previousHashes[key]
				if cont.StateHash != prevHash {
					log.Info().
						Str("container", key).
						Str("status", cont.Status.String()).
						Msg("Container status changed")

					statusChan <- StatusUpdate{
						ContainerID: key,
						Changed:     true,
					}
					previousHashes[key] = cont.StateHash
				}

				// 自動停止判定
				settings := appState.GetSettings()
				cfg, ok := settings.RegisteredContainers[key]
				if !ok || !cfg.AutoShutdown || cont.Status != container.StatusRunning {
					continue
				}

				// 稼働中でプレイヤーゼロの場合
				if cont.Status == container.StatusRunning && len(cont.Players) == 0 && !cont.StopTimer.IsZero() {
					elapsed := time.Since(cont.StopTimer)
					threshold := time.Duration(settings.RegularTask.AutoShutdownDelay) * time.Second

					if elapsed >= threshold {
						log.Info().
							Str("container", key).
							Dur("elapsed", elapsed).
							Msg("Auto-stopping container (no players)")

						cmdChan <- Command{
							Type:        "stop",
							ContainerID: key,
							Timeout:     10,
						}
					}
				}
			}
		}
	}
}
