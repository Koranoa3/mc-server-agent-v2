package discord

import (
	"fmt"
	"strings"

	"github.com/Koranoa3/mc-server-agent/internal/docker/container"
	"github.com/Koranoa3/mc-server-agent/internal/routine"
	"github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog/log"
)

// handleCommand はスラッシュコマンドを処理
func (b *Bot) handleCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	commandName := i.ApplicationCommandData().Name

	log.Info().
		Str("command", commandName).
		Str("user", i.Member.User.Username).
		Msg("Received command")

	switch commandName {
	case "mc-status":
		b.handleStatusCommand(s, i)
	case "mc-list":
		b.handleListCommand(s, i)
	case "mc-start":
		b.handleStartCommand(s, i)
	case "mc-stop":
		b.handleStopCommand(s, i)
	default:
		b.respondError(s, i, "Unknown command")
	}
}

// handleComponent はボタンやセレクトメニューを処理
func (b *Bot) handleComponent(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.MessageComponentData()
	customID := data.CustomID

	log.Info().
		Str("custom_id", customID).
		Str("user", i.Member.User.Username).
		Msg("Received component interaction")

	// CustomID の形式: "action:containerID" (例: "start:container1")
	parts := strings.SplitN(customID, ":", 2)
	if len(parts) != 2 {
		b.respondError(s, i, "Invalid button")
		return
	}

	action := parts[0]
	containerID := parts[1]

	switch action {
	case "start":
		b.executeCommand(s, i, "start", containerID)
	case "stop":
		b.executeCommand(s, i, "stop", containerID)
	case "refresh":
		b.handleRefreshButton(s, i)
	default:
		b.respondError(s, i, "Unknown action")
	}
}

// handleStatusCommand は /mc-status コマンドを処理
func (b *Bot) handleStatusCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	embed := b.buildStatusEmbed()

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
			Flags:  discordgo.MessageFlagsEphemeral,
		},
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to respond to status command")
	}
}

// handleListCommand は /mc-list コマンドを処理
func (b *Bot) handleListCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	embed := b.buildStatusEmbed()
	components := b.buildActionButtons()

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds:     []*discordgo.MessageEmbed{embed},
			Components: components,
		},
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to respond to list command")
	}
}

// handleStartCommand は /mc-start コマンドを処理
func (b *Bot) handleStartCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := i.ApplicationCommandData().Options
	if len(options) == 0 {
		b.respondError(s, i, "Server parameter is required")
		return
	}

	containerID := options[0].StringValue()
	b.executeCommand(s, i, "start", containerID)
}

// handleStopCommand は /mc-stop コマンドを処理
func (b *Bot) handleStopCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := i.ApplicationCommandData().Options
	if len(options) == 0 {
		b.respondError(s, i, "Server parameter is required")
		return
	}

	containerID := options[0].StringValue()
	b.executeCommand(s, i, "stop", containerID)
}

// handleRefreshButton は Refresh ボタンを処理
func (b *Bot) handleRefreshButton(s *discordgo.Session, i *discordgo.InteractionCreate) {
	embed := b.buildStatusEmbed()
	components := b.buildActionButtons()

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Embeds:     []*discordgo.MessageEmbed{embed},
			Components: components,
		},
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to update message on refresh")
	}
}

// executeCommand はコマンドを実行し結果を返す
func (b *Bot) executeCommand(s *discordgo.Session, i *discordgo.InteractionCreate, action, containerID string) {
	// 設定確認
	config, ok := b.settings.RegisteredContainers[containerID]
	if !ok {
		b.respondError(s, i, fmt.Sprintf("Container '%s' not found", containerID))
		return
	}

	// コンテナの現在状態をチェックして即時エラーメッセージを返す（起動済み・停止済み・プレイヤー在籍など）
	if stateObj, ok := b.appState.GetContainer(containerID); ok {
		if cont, ok := stateObj.(*container.Container); ok {
			switch action {
			case "start":
				if cont.Status == container.StatusRunning {
					b.respondError(s, i, fmt.Sprintf("%s is already running.", config.DisplayName))
					return
				}
				if cont.Status == container.StatusStarting {
					b.respondError(s, i, fmt.Sprintf("%s is currently starting. Please wait and try again.", config.DisplayName))
					return
				}
				if cont.Status == container.StatusNotFound || cont.ID == "" {
					b.respondError(s, i, fmt.Sprintf("%s is currently unavailable (container not found).", config.DisplayName))
					return
				}
			case "stop":
				if cont.Status == container.StatusStopped || cont.Status == container.StatusNotFound {
					b.respondError(s, i, fmt.Sprintf("%s is already stopped.", config.DisplayName))
					return
				}
				if len(cont.Players) > 0 {
					b.respondError(s, i, fmt.Sprintf("%s cannot be stopped because there are players online (%d players).", config.DisplayName, len(cont.Players)))
					return
				}
			}
		}
	} else {
		// state に存在しない場合は警告として返す
		b.respondError(s, i, fmt.Sprintf("Unable to retrieve status for %s. Please try again later.", config.DisplayName))
		return
	}

	// アクション確認
	if !b.isActionAllowed(action) {
		b.respondError(s, i, fmt.Sprintf("Sorry, the action `%s` is not allowed.", action))
		return
	}

	// Deferred response (処理に時間がかかるため)
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to send deferred response")
		return
	}

	// コマンドチャンネルに送信
	cmd := routine.Command{
		Type:        action,
		ContainerID: containerID,
		Timeout:     30,
	}

	select {
	case b.commandChan <- cmd:
		log.Info().
			Str("action", action).
			Str("container", containerID).
			Msg("Command sent to channel")

		// Followup メッセージで結果を通知
		content := fmt.Sprintf("✅ `%s` command sent to **%s**", action, config.DisplayName)
		_, err = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: content,
			Flags:   discordgo.MessageFlagsEphemeral,
		})

		if err != nil {
			log.Error().Err(err).Msg("Failed to send followup message")
		}

	default:
		log.Error().Msg("Command channel is full")
		_, err = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "❌ Command queue is full. Please try again later.",
			Flags:   discordgo.MessageFlagsEphemeral,
		})
		if err != nil {
			log.Error().Err(err).Msg("Failed to send error followup")
		}
	}
}

// isActionAllowed はアクションが許可されているか確認
func (b *Bot) isActionAllowed(action string) bool {
	switch action {
	case "start":
		return b.settings.AllowedActions.PowerOn
	case "stop":
		return b.settings.AllowedActions.PowerOff
	default:
		return false
	}
}

// respondError はエラーレスポンスを返す
func (b *Bot) respondError(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "❌ " + message,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})

	if err != nil {
		log.Error().Err(err).Str("message", message).Msg("Failed to send error response")
	}
}
