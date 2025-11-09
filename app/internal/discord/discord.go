package discord

import (
	"context"
	"fmt"
	"sync"

	"github.com/Koranoa3/mc-server-agent/internal/docker/container"
	"github.com/Koranoa3/mc-server-agent/internal/routine"
	"github.com/Koranoa3/mc-server-agent/internal/state"
	"github.com/Koranoa3/mc-server-agent/internal/utilities"
	"github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog/log"
)

// Bot は Discord Bot の管理構造体
type Bot struct {
	session     *discordgo.Session
	settings    *utilities.Settings
	appState    *state.AppState
	commandChan chan<- routine.Command
	guildID     string
	appID       string

	// コマンド登録情報
	commands           []*discordgo.ApplicationCommand
	registeredCommands []*discordgo.ApplicationCommand
	mu                 sync.RWMutex
}

// NewBot は新しい Discord Bot インスタンスを作成
func NewBot(token, guildID, appID string, settings *utilities.Settings, appState *state.AppState, commandChan chan<- routine.Command) (*Bot, error) {
	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("failed to create Discord session: %w", err)
	}

	bot := &Bot{
		session:     session,
		settings:    settings,
		appState:    appState,
		commandChan: commandChan,
		guildID:     guildID,
		appID:       appID,
	}

	// コマンド定義
	bot.defineCommands()

	// イベントハンドラー登録
	bot.registerHandlers()

	return bot, nil
}

// defineCommands はスラッシュコマンドを定義
func (b *Bot) defineCommands() {
	b.commands = []*discordgo.ApplicationCommand{
		{
			Name:        "mc-status",
			Description: "Show status of all Minecraft servers",
			NameLocalizations: &map[discordgo.Locale]string{
				discordgo.Japanese: "mc-ステータス",
			},
			DescriptionLocalizations: &map[discordgo.Locale]string{
				discordgo.Japanese: "全てのMinecraftサーバーの状態を表示",
			},
		},
		{
			Name:        "mc-list",
			Description: "Show list of all registered containers with buttons",
			NameLocalizations: &map[discordgo.Locale]string{
				discordgo.Japanese: "mc-リスト",
			},
			DescriptionLocalizations: &map[discordgo.Locale]string{
				discordgo.Japanese: "登録されているコンテナ一覧をボタン付きで表示",
			},
		},
		{
			Name:        "mc-start",
			Description: "Start a Minecraft server",
			NameLocalizations: &map[discordgo.Locale]string{
				discordgo.Japanese: "mc-起動",
			},
			DescriptionLocalizations: &map[discordgo.Locale]string{
				discordgo.Japanese: "Minecraftサーバーを起動",
			},
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "server",
					Description: "Server to start",
					NameLocalizations: map[discordgo.Locale]string{
						discordgo.Japanese: "サーバー",
					},
					DescriptionLocalizations: map[discordgo.Locale]string{
						discordgo.Japanese: "起動するサーバー",
					},
					Required: true,
					Choices:  b.buildServerChoices(),
				},
			},
		},
		{
			Name:        "mc-stop",
			Description: "Stop a Minecraft server",
			NameLocalizations: &map[discordgo.Locale]string{
				discordgo.Japanese: "mc-停止",
			},
			DescriptionLocalizations: &map[discordgo.Locale]string{
				discordgo.Japanese: "Minecraftサーバーを停止",
			},
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "server",
					Description: "Server to stop",
					NameLocalizations: map[discordgo.Locale]string{
						discordgo.Japanese: "サーバー",
					},
					DescriptionLocalizations: map[discordgo.Locale]string{
						discordgo.Japanese: "停止するサーバー",
					},
					Required: true,
					Choices:  b.buildServerChoices(),
				},
			},
		},
		{
			Name:        "whitelist",
			Description: "Manage Minecraft whitelist",
			NameLocalizations: &map[discordgo.Locale]string{
				discordgo.Japanese: "ホワイトリスト",
			},
			DescriptionLocalizations: &map[discordgo.Locale]string{
				discordgo.Japanese: "Minecraftホワイトリストを管理",
			},
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "add",
					Description: "Add a player to the whitelist",
					NameLocalizations: map[discordgo.Locale]string{
						discordgo.Japanese: "追加",
					},
					DescriptionLocalizations: map[discordgo.Locale]string{
						discordgo.Japanese: "プレイヤーをホワイトリストに追加",
					},
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "playername",
							Description: "Player name to add",
							NameLocalizations: map[discordgo.Locale]string{
								discordgo.Japanese: "プレイヤー名",
							},
							DescriptionLocalizations: map[discordgo.Locale]string{
								discordgo.Japanese: "追加するプレイヤー名",
							},
							Required: true,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "remove",
					Description: "Remove a player from the whitelist (Admin only)",
					NameLocalizations: map[discordgo.Locale]string{
						discordgo.Japanese: "削除",
					},
					DescriptionLocalizations: map[discordgo.Locale]string{
						discordgo.Japanese: "プレイヤーをホワイトリストから削除（管理者のみ）",
					},
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "playername",
							Description: "Player name to remove",
							NameLocalizations: map[discordgo.Locale]string{
								discordgo.Japanese: "プレイヤー名",
							},
							DescriptionLocalizations: map[discordgo.Locale]string{
								discordgo.Japanese: "削除するプレイヤー名",
							},
							Required: true,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "list",
					Description: "Show the whitelist (Admin only)",
					NameLocalizations: map[discordgo.Locale]string{
						discordgo.Japanese: "リスト",
					},
					DescriptionLocalizations: map[discordgo.Locale]string{
						discordgo.Japanese: "ホワイトリストを表示（管理者のみ）",
					},
				},
			},
		},
	}
}

// buildServerChoices は設定から選択肢を構築（存在しないコンテナは除外）
func (b *Bot) buildServerChoices() []*discordgo.ApplicationCommandOptionChoice {
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, len(b.settings.RegisteredContainers))

	for id, config := range b.settings.RegisteredContainers {
		// コンテナの存在確認
		if stateObj, ok := b.appState.GetContainer(id); ok {
			if cont, ok := stateObj.(*container.Container); ok {
				// StatusNotFound のコンテナは選択肢に含めない
				if cont.Status == container.StatusNotFound {
					continue
				}
			}
		}

		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  config.DisplayName,
			Value: id,
		})
	}

	return choices
}

// registerHandlers はイベントハンドラーを登録
func (b *Bot) registerHandlers() {
	// Ready イベント
	b.session.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		log.Info().
			Str("username", s.State.User.Username).
			Str("discriminator", s.State.User.Discriminator).
			Msg("Discord bot is ready")
		
		// 初期プレゼンスを設定
		b.UpdatePresence()
	})

	// Interaction Create イベント
	b.session.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		b.handleInteraction(s, i)
	})
}

// handleInteraction はインタラクションを処理
func (b *Bot) handleInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		b.handleCommand(s, i)
	case discordgo.InteractionMessageComponent:
		b.handleComponent(s, i)
	}
}

// Start は Discord Bot を起動
func (b *Bot) Start(ctx context.Context) error {
	// セッションを開く
	if err := b.session.Open(); err != nil {
		return fmt.Errorf("failed to open Discord session: %w", err)
	}

	log.Info().Msg("Discord session opened")

	// コマンドを登録
	if err := b.RegisterCommands(); err != nil {
		b.session.Close()
		return fmt.Errorf("failed to register commands: %w", err)
	}

	return nil
}

// Stop は Discord Bot を停止
func (b *Bot) Stop() error {
	log.Info().Msg("Stopping Discord bot")

	// コマンドを削除
	if err := b.UnregisterCommands(); err != nil {
		log.Error().Err(err).Msg("Failed to unregister commands")
	}

	// セッションを閉じる
	if err := b.session.Close(); err != nil {
		return fmt.Errorf("failed to close Discord session: %w", err)
	}

	log.Info().Msg("Discord bot stopped")
	return nil
}

// RegisterCommands はスラッシュコマンドを Discord に登録
func (b *Bot) RegisterCommands() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	log.Info().Int("count", len(b.commands)).Msg("Registering Discord commands")

	b.registeredCommands = make([]*discordgo.ApplicationCommand, 0, len(b.commands))

	for _, cmd := range b.commands {
		registered, err := b.session.ApplicationCommandCreate(b.appID, b.guildID, cmd)
		if err != nil {
			return fmt.Errorf("failed to register command '%s': %w", cmd.Name, err)
		}
		b.registeredCommands = append(b.registeredCommands, registered)
		log.Info().Str("name", cmd.Name).Str("id", registered.ID).Msg("Command registered")
	}

	return nil
}

// UnregisterCommands は登録したスラッシュコマンドを削除
func (b *Bot) UnregisterCommands() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.registeredCommands) == 0 {
		return nil
	}

	log.Info().Int("count", len(b.registeredCommands)).Msg("Unregistering Discord commands")

	for _, cmd := range b.registeredCommands {
		if err := b.session.ApplicationCommandDelete(b.appID, b.guildID, cmd.ID); err != nil {
			log.Error().Err(err).Str("name", cmd.Name).Msg("Failed to delete command")
		} else {
			log.Info().Str("name", cmd.Name).Msg("Command deleted")
		}
	}

	b.registeredCommands = nil
	return nil
}

// Session は Discord セッションを返す
func (b *Bot) Session() *discordgo.Session {
	return b.session
}

// UpdatePinnedMessages は📌リアクションがついた Bot のメッセージをすべて更新
func (b *Bot) UpdatePinnedMessages() {
	log.Info().Msg("Updating pinned messages")

	// すべての登録済みチャンネルを取得
	// guildID からギルドの全チャンネルを取得
	channels, err := b.session.GuildChannels(b.guildID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get guild channels")
		return
	}

	updatedCount := 0

	// 各チャンネルでピン留めされたメッセージを確認
	for _, channel := range channels {
		// テキストチャンネルのみ対象
		if channel.Type != discordgo.ChannelTypeGuildText {
			continue
		}

		// 各チャンネルのピン留めメッセージを取得
		messages, err := b.session.ChannelMessagesPinned(channel.ID)
		if err != nil {
			log.Error().Err(err).Str("channel_id", channel.ID).Msg("Failed to get channel messages")
			continue
		}

		// 各メッセージを確認
		for _, msg := range messages {
			// Bot 自身のメッセージかつ、Embeds または Components があるかチェック
			if msg.Author.ID != b.session.State.User.ID || (len(msg.Embeds) == 0 && len(msg.Components) == 0) {
				continue
			}

			// メッセージを更新
			embed := b.buildStatusEmbed()
			components := b.buildActionButtons()

			_, err := b.session.ChannelMessageEditComplex(&discordgo.MessageEdit{
				Channel:    msg.ChannelID,
				ID:         msg.ID,
				Embeds:     &[]*discordgo.MessageEmbed{embed},
				Components: &components,
			})

			if err != nil {
				log.Error().
					Err(err).
					Str("channel_id", msg.ChannelID).
					Str("message_id", msg.ID).
					Msg("Failed to update pinned message")
			} else {
				log.Debug().
					Str("channel_id", msg.ChannelID).
					Str("message_id", msg.ID).
					Msg("Updated pinned message")
				updatedCount++
			}
		}
	}

	log.Info().
		Int("updated_messages", updatedCount).
		Msg("Pinned messages update completed")
}
