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
