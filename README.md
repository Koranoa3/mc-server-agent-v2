# mc-server-agent-v2

**Minecraft サーバー管理用 Discord Bot (Go言語実装)**

Docker コンテナで稼働する Minecraft サーバーを Discord から操作・監視できる Bot です。

## 機能

- ✅ **Discord スラッシュコマンド**
  - `/mc-status` - サーバー状態の一覧表示
  - `/mc-list` - ボタン付き操作パネル表示
  - `/mc-start` - サーバー起動
  - `/mc-stop` - サーバー停止
  - `/mc-restart` - サーバー再起動

- ✅ **自動監視**
  - 定期的なコンテナ状態チェック
  - プレイヤー数に基づく自動停止機能

- ✅ **設定管理**
  - `settings.json` で複数サーバー管理
  - 環境変数 (`.env`) で機密情報を安全に管理

## セットアップ

### 1. リポジトリをクローン

```bash
git clone <repository-url>
cd mc-server-agent-v2
```

### 2. 環境変数の設定

`.env.example` をコピーして `.env` を作成:

```bash
cp .env.example .env
```

`.env` を編集して Discord Bot の認証情報を設定:

```bash
DISCORD_BOT_TOKEN=your_bot_token_here
DISCORD_GUILD_ID=your_guild_id_here
DISCORD_APP_ID=your_application_id_here
```

### 3. 設定ファイルの作成

`settings.example.json` をコピーして `settings.json` を作成:

```bash
cp settings.example.json settings.json
```

`settings.json` を編集してコンテナ情報を登録:

```json
{
  "registered_containers": {
    "container_id_1": {
      "display_name": "Survival Server",
      "container_name": "minecraft_survival",
      "path": "/data/minecraft/survival",
      "icon": "⛏️",
      "auto_shutdown": true
    }
  }
}
```

### 4. Discord Bot の作成

1. [Discord Developer Portal](https://discord.com/developers/applications) でアプリケーションを作成
2. Bot タブで Bot を作成し、トークンを取得
3. OAuth2 タブで以下の権限を付与:
   - `bot`
   - `applications.commands`
4. Bot Permissions で以下を選択:
   - Send Messages
   - Use Slash Commands
5. 生成された URL でサーバーに招待

### 5. 起動

#### Docker Compose で起動 (推奨)

```bash
docker compose up --build
```

#### ローカルで起動 (開発用)

```bash
cd app
go build -v -o ../mc-agent
cd ..
./mc-agent
```

## Docker Socket へのアクセス

このアプリケーションは Docker API を使用してコンテナを操作します。

### セキュリティ設定

`docker-compose.yml` で Docker socket をマウントしています:

```yaml
volumes:
  - /var/run/docker.sock:/var/run/docker.sock:rw
```

**セキュリティ上の注意:**
- Docker socket へのアクセスは root 権限相当です
- 本番環境では以下の対策を検討してください:
  - Docker TLS 認証の使用
  - SSH トンネル経由でのアクセス
  - Docker-in-Docker (DinD) の使用

## 使い方

### Discord コマンド

1. **ステータス確認**
   ```
   /mc-status
   ```
   全サーバーの状態を一覧表示

2. **操作パネル表示**
   ```
   /mc-list
   ```
   ボタンで操作できるパネルを表示

3. **サーバー起動**
   ```
   /mc-start server:サーバー名
   ```

4. **サーバー停止**
   ```
   /mc-stop server:サーバー名
   ```

5. **サーバー再起動**
   ```
   /mc-restart server:サーバー名
   ```

## アーキテクチャ

詳細は [STRUCTURE.md](./STRUCTURE.md) を参照してください。

### 主要コンポーネント

- **main.go** - メディエーターパターンによるイベント管理
- **discord/** - Discord Bot インタラクション処理
- **docker/** - Docker API ラッパー
- **routine/** - 定期監視と自動停止ロジック
- **state/** - スレッドセーフな状態管理
- **utilities/** - 設定/ログ管理

## 開発

### 必要な環境

- Go 1.24+
- Docker & Docker Compose
- Discord Bot アカウント

### ビルド

```bash
cd app
go build -v -o ../mc-agent
```

### テスト実行

```bash
cd app
go test ./...
```

## トラブルシューティング

### Docker socket の権限エラー

```
permission denied while trying to connect to the Docker daemon socket
```

**解決方法:**

1. `.env` で UID/GID を設定:
   ```bash
   UID=$(id -u)
   DOCKER_GID=$(getent group docker | cut -d: -f3)
   ```

2. コンテナを再起動:
   ```bash
   docker compose down
   docker compose up --build
   ```

### Discord Bot が起動しない

- `.env` の `DISCORD_BOT_TOKEN` が正しいか確認
- Bot に必要な権限が付与されているか確認
- ログを確認: `docker logs mc-server-agent`

### 設定ファイルの読み込みエラー

- `settings.json` のフォーマットが正しいか確認
- `SETTINGS_PATH` 環境変数が正しいか確認

## ライセンス

MIT License

## 貢献

Pull Request を歓迎します!

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

