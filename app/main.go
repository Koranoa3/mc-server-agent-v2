package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Koranoa3/mc-server-agent/internal/docker"
	"github.com/Koranoa3/mc-server-agent/internal/routine"
	"github.com/Koranoa3/mc-server-agent/internal/state"
	"github.com/Koranoa3/mc-server-agent/internal/utilities"
	"github.com/rs/zerolog/log"
)

func main() {
	// 設定ファイルの読み込み
	settingsPath := os.Getenv("SETTINGS_PATH")
	if settingsPath == "" {
		settingsPath = "settings.json"
	}

	settings, err := utilities.LoadSettings(settingsPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load settings: %v\n", err)
		os.Exit(1)
	}

	// ロガー初期化
	utilities.InitLogger(settings.LogLevel)
	log.Info().Msg("Application starting")

	// アプリケーション状態の初期化
	appState := state.NewAppState(settings)

	// Docker マネージャーの初期化
	dockerManager, err := docker.NewManager(appState)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create docker manager")
	}
	defer dockerManager.Close()

	// Context と graceful shutdown の準備
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// シグナルハンドリング
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	// Channel の作成
	commandChan := make(chan routine.Command, 10)
	statusUpdateChan := make(chan routine.StatusUpdate, 10)
	errorChan := make(chan error, 10)

	// 初期コンテナ情報取得
	log.Info().Msg("Fetching initial container information")
	if err := dockerManager.UpdateAllContainers(ctx); err != nil {
		log.Error().Err(err).Msg("Failed to update containers")
	} else {
		containers := appState.GetAllContainers()
		log.Info().Int("count", len(containers)).Msg("Containers loaded")
	}

	// Routine goroutine の起動
	go routine.Run(ctx, appState, dockerManager, statusUpdateChan, commandChan)

	// メインループ
	log.Info().Msg("Entering main event loop")
	ticker := time.NewTicker(time.Duration(settings.RegularTask.Interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Context cancelled, shutting down")
			return

		case sig := <-sigCh:
			log.Info().Str("signal", sig.String()).Msg("Received shutdown signal")
			cancel()
			return

		case cmd := <-commandChan:
			log.Info().
				Str("type", cmd.Type).
				Str("container", cmd.ContainerID).
				Msg("Processing command")

			switch cmd.Type {
			case "start":
				if err := dockerManager.StartContainer(ctx, cmd.ContainerID); err != nil {
					log.Error().Err(err).Str("container", cmd.ContainerID).Msg("Failed to start container")
					errorChan <- err
				} else {
					log.Info().Str("container", cmd.ContainerID).Msg("Container started")
				}

			case "stop":
				timeout := cmd.Timeout
				if timeout == 0 {
					timeout = 10
				}
				if err := dockerManager.StopContainer(ctx, cmd.ContainerID, timeout); err != nil {
					log.Error().Err(err).Str("container", cmd.ContainerID).Msg("Failed to stop container")
					errorChan <- err
				} else {
					log.Info().Str("container", cmd.ContainerID).Msg("Container stopped")
				}

			case "restart":
				timeout := cmd.Timeout
				if timeout == 0 {
					timeout = 10
				}
				if err := dockerManager.RestartContainer(ctx, cmd.ContainerID, timeout); err != nil {
					log.Error().Err(err).Str("container", cmd.ContainerID).Msg("Failed to restart container")
					errorChan <- err
				} else {
					log.Info().Str("container", cmd.ContainerID).Msg("Container restarted")
				}
			}

		case update := <-statusUpdateChan:
			log.Debug().
				Str("container", update.ContainerID).
				Bool("changed", update.Changed).
				Msg("Status update received")
			// TODO: Discord へ通知

		case err := <-errorChan:
			log.Error().Err(err).Msg("Error received")
			// TODO: Discord へエラー通知

		case <-ticker.C:
			log.Debug().Msg("Periodic container check")
			if err := dockerManager.UpdateAllContainers(ctx); err != nil {
				log.Error().Err(err).Msg("Failed to update containers")
			}
		}
	}
}
