package discord

import (
	"fmt"
	"regexp"
	"sort"
	"time"

	"github.com/Koranoa3/mc-server-agent/internal/docker/container"
	"github.com/bwmarrin/discordgo"
)

// parseEmoji はカスタム絵文字文字列をパースする
// 形式: <:name:id> または <a:name:id>
func parseEmoji(emojiStr string) *discordgo.ComponentEmoji {
	// カスタム絵文字のパターン
	pattern := regexp.MustCompile(`<(a)?:([^:]+):(\d+)>`)
	matches := pattern.FindStringSubmatch(emojiStr)

	if len(matches) == 4 {
		// カスタム絵文字
		return &discordgo.ComponentEmoji{
			Name:     matches[2], // 絵文字名
			ID:       matches[3], // 絵文字ID
			Animated: matches[1] == "a",
		}
	}

	// Unicode 絵文字またはパース失敗時
	return &discordgo.ComponentEmoji{
		Name: emojiStr,
	}
}

// buildStatusEmbed はコンテナステータスの Embed を構築
func (b *Bot) buildStatusEmbed() *discordgo.MessageEmbed {
	containers := b.appState.GetAllContainers()

	fields := make([]*discordgo.MessageEmbedField, 0, len(containers))

	// コンテナをIDでソート
	ids := make([]string, 0, len(containers))
	for id := range containers {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		containerInterface := containers[id]
		config, ok := b.settings.RegisteredContainers[id]
		if !ok {
			continue
		}

		// interface{} から *container.Container に型アサーション
		cont, ok := containerInterface.(*container.Container)
		if !ok {
			continue
		}

		// ステータスアイコン
		statusIcon := b.getStatusIcon(cont.Status)
		statusText := cont.Status.JapaneseString()

		// アイコン取得
		icon := config.Icon
		if iconURL, ok := b.settings.Icons[icon]; ok {
			icon = iconURL
		}

		// フィールド値作成
		value := fmt.Sprintf("%s **%s**", statusIcon, statusText)

		// プレイヤー情報があれば追加
		if len(cont.Players) > 0 {
			value += fmt.Sprintf("\n👥 Players: %d", len(cont.Players))
		}

		// 自動停止設定
		if config.AutoShutdown {
			value += "\n⏱️ Auto-shutdown ON"
		}

		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   fmt.Sprintf("%s %s", icon, config.DisplayName),
			Value:  value,
			Inline: true,
		})
	}

	// フィールドがない場合
	if len(fields) == 0 {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "No Servers",
			Value:  "No registered servers found.",
			Inline: false,
		})
	}

	embed := &discordgo.MessageEmbed{
		Title:       "🖥️ Minecraft Server Status",
		Description: "Current status of all registered servers",
		Color:       0x00ff00, // Green
		Fields:      fields,
		Timestamp:   time.Now().Format(time.RFC3339),
		Footer: &discordgo.MessageEmbedFooter{
			Text: "MC Server Agent",
		},
	}

	return embed
}

// buildActionButtons はアクションボタンを構築
func (b *Bot) buildActionButtons() []discordgo.MessageComponent {
	if !b.settings.AllowedActions.PlaceButtons {
		return nil
	}

	containers := b.appState.GetAllContainers()
	rows := make([]discordgo.MessageComponent, 0)

	for id, containerInterface := range containers {
		config, ok := b.settings.RegisteredContainers[id]
		if !ok {
			continue
		}

		cont, ok := containerInterface.(*container.Container)
		if !ok {
			continue
		}

		// StatusNotFound のコンテナはボタンを表示しない
		if cont.Status == container.StatusNotFound {
			continue
		}

		buttons := []discordgo.MessageComponent{}

		// Start ボタン用の絵文字取得
		startEmoji := "▶️"
		if icon, ok := b.settings.Icons["poweron_mono"]; ok {
			startEmoji = icon
		}

		// Stop ボタン用の絵文字取得
		stopEmoji := "⏹️"
		if icon, ok := b.settings.Icons["poweroff_mono"]; ok {
			stopEmoji = icon
		}

		// Start ボタン
		if b.settings.AllowedActions.PowerOn && cont.Status != container.StatusRunning {
			buttons = append(buttons, discordgo.Button{
				Label:    "Start",
				Style:    discordgo.SuccessButton,
				CustomID: fmt.Sprintf("start:%s", id),
				Emoji:    parseEmoji(startEmoji),
			})
		}

		// Stop ボタン
		if b.settings.AllowedActions.PowerOff && cont.Status == container.StatusRunning {
			buttons = append(buttons, discordgo.Button{
				Label:    "Stop",
				Style:    discordgo.DangerButton,
				CustomID: fmt.Sprintf("stop:%s", id),
				Emoji:    parseEmoji(stopEmoji),
			})
		}

		// ボタンがある場合のみ行を追加
		if len(buttons) > 0 {
			// サーバー名ラベル追加
			labelButton := discordgo.Button{
				Label:    config.DisplayName,
				Style:    discordgo.SecondaryButton,
				CustomID: fmt.Sprintf("label:%s", id),
				Disabled: true,
			}

			// ラベルを先頭に追加
			buttonsWithLabel := append([]discordgo.MessageComponent{labelButton}, buttons...)

			rows = append(rows, discordgo.ActionsRow{
				Components: buttonsWithLabel,
			})
		}
	}

	// Refresh ボタンを最後に追加
	if len(rows) > 0 {
		// Refresh アイコン取得
		refreshEmoji := "🔄"
		if icon, ok := b.settings.Icons["reload_mono"]; ok {
			refreshEmoji = icon
		}

		rows = append(rows, discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    "Refresh Status",
					Style:    discordgo.SecondaryButton,
					CustomID: "refresh:all",
					Emoji:    parseEmoji(refreshEmoji),
				},
			},
		})
	}

	return rows
}

// getStatusIcon はステータスに対応する絵文字を返す
func (b *Bot) getStatusIcon(status container.WorkingStatus) string {
	switch status {
	case container.StatusRunning:
		if icon, ok := b.settings.Icons["poweron"]; ok {
			return icon
		}
		return "🟢"
	case container.StatusStarting:
		if icon, ok := b.settings.Icons["reload"]; ok {
			return icon
		}
		return "🟡"
	case container.StatusStopped:
		if icon, ok := b.settings.Icons["poweroff"]; ok {
			return icon
		}
		return "🔴"
	case container.StatusNotFound:
		if icon, ok := b.settings.Icons["deny"]; ok {
			return icon
		}
		return "❓"
	default:
		return "⚪"
	}
}

// buildServerSelectMenu はサーバー選択メニューを構築（未使用だが将来用）
func (b *Bot) buildServerSelectMenu() discordgo.SelectMenu {
	options := make([]discordgo.SelectMenuOption, 0, len(b.settings.RegisteredContainers))

	for id, config := range b.settings.RegisteredContainers {
		cont, ok := b.appState.GetContainer(id)
		if !ok {
			continue
		}

		emoji := "⚪"
		description := "Unknown status"

		if c, ok := cont.(*container.Container); ok {
			emoji = b.getStatusIcon(c.Status)
			description = c.Status.JapaneseString()
		}

		options = append(options, discordgo.SelectMenuOption{
			Label:       config.DisplayName,
			Value:       id,
			Description: description,
			Emoji:       parseEmoji(emoji),
		})
	}

	return discordgo.SelectMenu{
		CustomID:    "server_select",
		Placeholder: "Select a server...",
		MinValues:   func() *int { v := 1; return &v }(),
		MaxValues:   1,
		Options:     options,
	}
}
