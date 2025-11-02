package discord

import (
	"fmt"

	"github.com/Koranoa3/mc-server-agent/internal/docker/container"
	"github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog/log"
)

// UpdatePresence はBotのステータスメッセージを更新
func (b *Bot) UpdatePresence() {
	if b.session == nil {
		return
	}

	containers := b.appState.GetAllContainers()

	// オンラインプレイヤー数をカウント
	totalPlayers := 0
	runningCount := 0

	for _, containerInterface := range containers {
		cont, ok := containerInterface.(*container.Container)
		if !ok {
			continue
		}

		if cont.Status == container.StatusRunning {
			runningCount++
			totalPlayers += len(cont.Players)
		}
	}

	var status discordgo.Status
	var activityType discordgo.ActivityType
	var message string

	// プレイヤーがいる場合
	if totalPlayers > 0 {
		status = discordgo.StatusOnline
		activityType = discordgo.ActivityTypeGame
		message = fmt.Sprintf("%d人がプレイ中", totalPlayers)
	} else if runningCount > 0 {
		// 稼働中サーバーがある場合
		status = discordgo.StatusOnline
		activityType = discordgo.ActivityTypeWatching
		message = fmt.Sprintf("0人 | %d サーバーが稼働中", runningCount)
	} else {
		// 待機中
		status = discordgo.StatusIdle
		activityType = discordgo.ActivityTypeListening
		message = "/mc-list | 待機中"
	}

	err := b.session.UpdateStatusComplex(discordgo.UpdateStatusData{
		Status: string(status),
		Activities: []*discordgo.Activity{
			{
				Name: message,
				Type: activityType,
			},
		},
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to update presence")
	} else {
		log.Debug().
			Str("status", string(status)).
			Str("message", message).
			Msg("Presence updated")
	}
}
