package discord

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Koranoa3/mc-server-agent/internal/docker/container"
	"github.com/Koranoa3/mc-server-agent/internal/routine"
	"github.com/Koranoa3/mc-server-agent/internal/utilities"
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
	case "whitelist":
		b.handleWhitelistCommand(s, i)
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

		// Followup メッセージで結果を通知（自動削除をスケジュール）
		allow_icon := b.settings.Icons["allow"]
		content := fmt.Sprintf("%s `%s` command sent to **%s**", allow_icon, action, config.DisplayName)
		msg, err := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: content,
		})

		if err != nil {
			log.Error().Err(err).Msg("Failed to send followup message")
		} else {
			if b.settings != nil && b.settings.MessageDeleteAfter > 0 {
				go func(msg *discordgo.Message) {
					time.Sleep(time.Duration(b.settings.MessageDeleteAfter) * time.Second)
					if err := s.FollowupMessageDelete(i.Interaction, msg.ID); err != nil {
						log.Debug().Err(err).Msg("Failed to delete followup message")
					}
				}(msg)
			}
		}

	default:
		log.Error().Msg("Command channel is full")
		// エラーフォローアップ（自動削除）
		deny_icon := b.settings.Icons["deny"]
		msg, err := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: fmt.Sprintf("%s Command queue is full. Please try again later.", deny_icon),
		})
		if err != nil {
			log.Error().Err(err).Msg("Failed to send error followup")
		} else if b.settings != nil && b.settings.MessageDeleteAfter > 0 {
			go func(msg *discordgo.Message) {
				time.Sleep(time.Duration(b.settings.MessageDeleteAfter) * time.Second)
				if err := s.FollowupMessageDelete(i.Interaction, msg.ID); err != nil {
					log.Debug().Err(err).Msg("Failed to delete error followup")
				}
			}(msg)
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
	deny_icon := b.settings.Icons["deny"]
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("%s %s", deny_icon, message),
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})

	if err != nil {
		log.Error().Err(err).Str("message", message).Msg("Failed to send error response")
		return
	}

	// 自動削除スケジュール
	if b.settings != nil && b.settings.MessageDeleteAfter > 0 {
		go func() {
			time.Sleep(time.Duration(b.settings.MessageDeleteAfter) * time.Second)
			if derr := s.InteractionResponseDelete(i.Interaction); derr != nil {
				log.Debug().Err(derr).Msg("Failed to delete interaction response (error)")
			}
		}()
	}
}

// handleWhitelistCommand は /whitelist コマンドを処理
func (b *Bot) handleWhitelistCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := i.ApplicationCommandData().Options
	if len(options) == 0 {
		b.respondError(s, i, "Subcommand is required")
		return
	}

	subcommand := options[0]

	switch subcommand.Name {
	case "add":
		b.handleWhitelistAdd(s, i, subcommand)
	case "remove":
		b.handleWhitelistRemove(s, i, subcommand)
	case "list":
		b.handleWhitelistList(s, i)
	default:
		b.respondError(s, i, "Unknown subcommand")
	}
}

// handleWhitelistAdd はプレイヤーをホワイトリストに追加
func (b *Bot) handleWhitelistAdd(s *discordgo.Session, i *discordgo.InteractionCreate, subcommand *discordgo.ApplicationCommandInteractionDataOption) {
	if len(subcommand.Options) == 0 {
		b.respondError(s, i, "Player name is required")
		return
	}

	playerName := subcommand.Options[0].StringValue()
	whitelistPath := os.Getenv("WHITELIST_PATH")
	if whitelistPath == "" {
		b.respondError(s, i, "WHITELIST_PATH environment variable is not set")
		return
	}

	// Deferred response
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

	// Mojang API でプレイヤー情報を取得
	profile, err := utilities.FetchMojangProfile(playerName)
	if err != nil {
		deny_icon := b.settings.Icons["deny"]
		content := fmt.Sprintf("%s プレイヤー名が不明です: %s", deny_icon, playerName)
		if err.Error() != "player not found" {
			content = fmt.Sprintf("%s エラーが発生しました: %v", deny_icon, err)
		}

		s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: content,
		})
		return
	}

	// ホワイトリストに追加
	userID := i.Member.User.ID
	isNew, err := utilities.AddToWhitelist(whitelistPath, profile.ID, profile.Name, userID)
	if err != nil {
		deny_icon := b.settings.Icons["deny"]
		s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: fmt.Sprintf("%s エラーが発生しました: %v", deny_icon, err),
		})
		return
	}

	// 成功メッセージ
	allow_icon := b.settings.Icons["allow"]
	var content string
	if isNew {
		content = fmt.Sprintf("%s **%s** を追加しました", allow_icon, profile.Name)
	} else {
		content = fmt.Sprintf("%s **%s** は既にホワイトリストに含まれています", allow_icon, profile.Name)
	}

	s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: content,
	})

	// 稼働中のコンテナにホワイトリスト更新を通知
	b.refreshAllContainersWhitelist()
}

// handleWhitelistRemove はプレイヤーをホワイトリストから削除
func (b *Bot) handleWhitelistRemove(s *discordgo.Session, i *discordgo.InteractionCreate, subcommand *discordgo.ApplicationCommandInteractionDataOption) {
	// 管理者権限チェック
	if !b.isAdmin(i.Member) {
		b.respondError(s, i, "この操作には管理者権限が必要です")
		return
	}

	if len(subcommand.Options) == 0 {
		b.respondError(s, i, "Player name is required")
		return
	}

	playerName := subcommand.Options[0].StringValue()
	whitelistPath := os.Getenv("WHITELIST_PATH")
	if whitelistPath == "" {
		b.respondError(s, i, "WHITELIST_PATH environment variable is not set")
		return
	}

	// Deferred response
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

	// Mojang API でプレイヤー情報を取得
	profile, err := utilities.FetchMojangProfile(playerName)
	if err != nil {
		deny_icon := b.settings.Icons["deny"]
		content := fmt.Sprintf("%s プレイヤー名が不明です: %s", deny_icon, playerName)
		if err.Error() != "player not found" {
			content = fmt.Sprintf("%s エラーが発生しました: %v", deny_icon, err)
		}

		s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: content,
		})
		return
	}

	// ホワイトリストから削除
	removed, err := utilities.RemoveFromWhitelist(whitelistPath, profile.ID)
	if err != nil {
		deny_icon := b.settings.Icons["deny"]
		s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: fmt.Sprintf("%s エラーが発生しました: %v", deny_icon, err),
		})
		return
	}

	// 成功メッセージ
	allow_icon := b.settings.Icons["allow"]
	var content string
	if removed {
		content = fmt.Sprintf("%s **%s** を削除しました", allow_icon, profile.Name)
	} else {
		content = fmt.Sprintf("%s **%s** はホワイトリストに含まれていません", allow_icon, profile.Name)
	}

	s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: content,
	})

	// 稼働中のコンテナにホワイトリスト更新を通知
	b.refreshAllContainersWhitelist()
}

// handleWhitelistList はホワイトリストを表示
func (b *Bot) handleWhitelistList(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// 管理者権限チェック
	if !b.isAdmin(i.Member) {
		b.respondError(s, i, "この操作には管理者権限が必要です")
		return
	}

	whitelistPath := os.Getenv("WHITELIST_PATH")
	if whitelistPath == "" {
		b.respondError(s, i, "WHITELIST_PATH environment variable is not set")
		return
	}

	// ホワイトリストを読み込み
	entries, err := utilities.LoadWhitelist(whitelistPath)
	if err != nil {
		b.respondError(s, i, fmt.Sprintf("エラーが発生しました: %v", err))
		return
	}

	// リストを整形
	var builder strings.Builder
	builder.WriteString("```\n")
	builder.WriteString(fmt.Sprintf("Total: %d players\n\n", len(entries)))

	for i, entry := range entries {
		addedBy := "Unknown"
		if entry.AddedUserID != "" {
			addedBy = fmt.Sprintf("<@%s>", entry.AddedUserID)
		}
		builder.WriteString(fmt.Sprintf("%d. %s (Added by: %s)\n", i+1, entry.Name, addedBy))
	}

	builder.WriteString("```")

	// レスポンスを送信 (ephemeral, message_deleteafter は適用しない)
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: builder.String(),
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to respond to whitelist list command")
	}
}

// isAdmin は管理者権限をチェック
func (b *Bot) isAdmin(member *discordgo.Member) bool {
	// Administrator 権限を持っているかチェック
	permissions := int64(member.Permissions)
	return (permissions & discordgo.PermissionAdministrator) != 0
}

// refreshAllContainersWhitelist はすべての稼働中コンテナのホワイトリストを更新
func (b *Bot) refreshAllContainersWhitelist() {
	containers := b.appState.GetAllContainers()
	ctx := context.Background()

	for id, containerInterface := range containers {
		cont, ok := containerInterface.(*container.Container)
		if !ok {
			continue
		}

		// StatusRunning のコンテナのみ更新
		if cont.Status != container.StatusRunning {
			continue
		}

		if err := cont.RefreshWhitelist(ctx); err != nil {
			log.Error().
				Err(err).
				Str("container_id", id).
				Msg("Failed to refresh whitelist")
		} else {
			log.Info().
				Str("container_id", id).
				Msg("Whitelist refreshed")
		}
	}
}
