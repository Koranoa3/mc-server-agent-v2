# アプリケーションの構造

## ディレクトリ構成

```
app/
	main.go
	internal/
		state/
			state.go
		discord/
			discord.go
			handlers.go
			components.go
			formatter/
				status_message.go
				container_list.go
		docker/
			docker.go
			container/
				container.go
				status.go
				players.go
		routine/
			routine.go
		utilities/
			settings.go
			logger.go
	go.mod
	go.sum
settings.json
.env
```

## goモジュール構成

**main.go**
- アプリケーションのエントリポイント。
- 各モジュール（discord, docker, routine）のインスタンス作成と初期化。
- channel を使った疎結合な通信を仲介（mediator パターン）。
- graceful shutdown 処理（context キャンセル）。
- メインループ: 各 channel からのイベントを受信して適切なモジュールに振り分け。

**channel 通信の設計** (循環依存回避):
```
main.go が以下の channel を管理:
  - statusUpdateChan: routine → main → discord (状態変化通知)
  - commandChan: discord → main → docker (ユーザー操作)
  - errorChan: 全モジュール → main (エラー集約)
```

各モジュールは他モジュールを直接参照せず、channel 経由でのみ通信する。

### state

**state.go**
- **目的**: アプリケーション全体のグローバル状態を一元管理（スレッドセーフ）。
- **責務**:
  - コンテナ情報のキャッシュ（ID/名前 → Container オブジェクトのマップ）
  - 設定情報（Settings struct）の保持
  - プレイヤー情報やホワイトリストの状態管理
- **実装**:
  - `sync.RWMutex` で排他制御（複数 goroutine からの安全なアクセス）
  - Getter/Setter メソッドでカプセル化
  - 状態変更時に変更検知フラグをセット（routine が監視）
- **依存**: utilities/settings からロード、docker からコンテナ情報更新

```go
type AppState struct {
    mu          sync.RWMutex
    settings    *Settings
    containers  map[string]*container.Container
}

func (s *AppState) GetContainer(id string) (*container.Container, bool)
func (s *AppState) UpdateContainer(id string, c *container.Container)
func (s *AppState) GetSettings() *Settings
```

### discord

**discord.go**
- **責務**: Discord セッション管理、イベントハンドラ登録、スラッシュコマンド登録。
- **初期化**: トークンでセッション作成 → Ready イベント待機 → コマンド登録。
- **受信**: ユーザーからのインタラクション（コマンド、ボタンクリック）を受け取る。
- **送信**: ステータス更新メソッド（main.go から呼ばれる）でメッセージ編集/送信。
- **依存**: main.go が渡す commandChan にコマンドを送信（docker は直接参照しない）。

**handlers.go**
- **責務**: インタラクションハンドラの実装。
- **処理フロー**:
  1. Discord からボタンクリック/コマンド受信
  2. パラメータ検証
  3. commandChan に Command 構造体を送信（main.go が処理）
  4. 結果を Discord に返答（ephemeral メッセージ or メッセージ更新）
- **依存**: components.go で UI 生成、formatter で整形。

**components.go**
- **責務**: Discord UI コンポーネント（ボタン、セレクト、Embed）の生成。
- **機能**:
  - settingsに登録されたコンテナが複数ある場合、コンテナ選択ボタンを生成
  - 「すべてのコンテナの状態を表示」ボタンも配置
  - コンテナが選択されたら、そのコンテナに対する操作ボタンを生成（状態に応じて起動/停止）
  - Custom ID の生成（コンテナID、操作種別を含む）
- **依存**: state から現在の状態を取得してボタンの有効/無効を決定。

#### discord/formatter

**status_message.go**
- **責務**: 受け取ったコンテナ状態データを Discord Embed 形式に整形する。
- **入力**: state から取得した Container オブジェクト配列。
- **出力**: Discord MessageEmbed（色、フィールド、アイコン付き）。
- **機能**: 稼働状況に応じた色分け、プレイヤー数表示、アイコン埋め込み。

**container_list.go**
- **責務**: コンテナ一覧を Discord メッセージ形式に整形する。
- **入力**: Container オブジェクト配列。
- **出力**: テキストまたは Embed（一覧表示用）。
- **機能**: 各コンテナの名前、状態、プレイヤー数を簡潔に表示。

### docker

**docker.go**
- **責務**: Docker API とのインターフェース、コンテナ管理。
- **初期化**: Docker client 作成（環境変数 DOCKER_HOST から接続先取得）。
- **機能**:
  - settingsに登録されたコンテナを Docker API から取得（稼働中/停止中/存在しない）
  - コンテナの起動/停止/再起動命令の実行
  - コンテナ情報を Container オブジェクトとして返す
- **依存**: 
  - state から設定情報取得
  - 取得したコンテナ情報を state に保存
  - main.go の commandChan から操作命令を受信
- **実装**: ContainerManager interface を定義してテスト可能に

```go
type ContainerManager interface {
    List(ctx context.Context) ([]*container.Container, error)
    Start(ctx context.Context, id string) error
    Stop(ctx context.Context, id string) error
    Inspect(ctx context.Context, id string) (*container.Container, error)
}
```

#### docker/container

**container.go**
- **責務**: コンテナオブジェクトの定義と基本操作。
- **構造体**:
  ```go
  type Container struct {
      ID           string
      Name         string
      Status       WorkingStatus
      Image        string
      Health       string
      Players      int
      LastChecked  time.Time
      StateHash    string  // 変更検知用ハッシュ
  }
  ```
- **機能**:
  - Docker client をラップし、各コンテナの操作を行う
  - 開始/停止/再起動メソッド
  - 状態を保持し、ハッシュ化して変更検知に使う（前回と比較）
  - コンテナ情報の更新（Inspect API から最新情報取得）
- **依存**: Docker client、status.go の WorkingStatus。

**status.go**
- **責務**: 各コンテナの稼働状況を定義する enum と判定ロジック。
- **定義**:
  ```go
  type WorkingStatus int
  const (
      StatusUnknown WorkingStatus = iota
      StatusRunning      // 稼働中
      StatusStarting     // 起動中（ヘルスチェック待機）
      StatusStopped      // 停止中
      StatusNotFound     // 存在しない
  )
  ```
- **機能**:
  - Container Inspect の State と Health から WorkingStatus を判定
  - Minecraft 特有の「起動中」ステータスは Healthcheck の Status を参照
  - String() メソッドで人間可読な文字列に変換

**players.go**
- **責務**: Minecraft のプレイヤーリスト取得とパース。
- **機能**:
  - RCON または Container Exec で `list` コマンド実行
  - コマンド出力をパースして Player 構造体配列に変換
  - エラーハンドリング（コンテナ停止中、RCON 接続失敗等）
- **実装**:
  ```go
  type Player struct {
      Name string
      UUID string
  }

  func GetPlayers(ctx context.Context, containerID string) (int, error)
  ```
- **依存**: Docker client（exec）または RCON ライブラリ（別途検討）。

### routine

**routine.go**
- **責務**: 定期的なコンテナ監視と自動停止判定。
- **実装**: 独立した goroutine で稼働、context でキャンセル可能。
- **処理フロー**:
  1. settings から interval（秒）を取得
  2. ticker で定期実行
  3. docker.List() でコンテナ情報取得
  4. 前回の状態と比較（ハッシュ値）
  5. 変更があれば statusUpdateChan に送信（main → discord が受信）
  6. プレイヤー数ゼロ＆設定時間以上経過したコンテナを検出
  7. auto_shutdown が true なら停止命令を commandChan に送信
- **依存**: 
  - state から設定と前回状態を取得
  - docker を呼び出して最新情報取得
  - channel 経由で main.go に通知（discord/docker は直接参照しない）
- **注意**: race condition を避けるため state へのアクセスは mutex 経由

```go
func Run(ctx context.Context, state *AppState, statusChan chan<- StatusUpdate, cmdChan chan<- Command) {
    ticker := time.NewTicker(time.Duration(state.GetSettings().Interval) * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            // 定期チェック処理
        }
    }
}
```

### utilities

**settings.go**
- **責務**: 設定ファイル（settings.json）の読み書きと構造体化。
- **機能**:
  - SETTINGS_PATH 環境変数から読み込み（デフォルト: `/data/settings.json`）
  - JSON を Settings 構造体にパース
  - バリデーション（必須フィールドチェック、interval > 0 等）
  - atomic 書き込み（tmp ファイル作成 → rename）で破損防止
  - flock でファイルロック（並列プロセスからの競合対策）
- **構造体定義**:
  ```go
  type Settings struct {
      LogLevel             string                       `json:"log_level"`
      RegularTask          RegularTaskConfig            `json:"regular_task"`
      RegisteredContainers map[string]ContainerConfig   `json:"registered_containers"`
      MessageDeleteAfter   int                          `json:"message_deleteafter"`
      AllowedActions       AllowedActions               `json:"allowed_actions"`
      Icons                map[string]string            `json:"icons"`
  }
  
  type ContainerConfig struct {
      DisplayName   string `json:"display_name"`
      ContainerName string `json:"container_name"`
      Path          string `json:"path"`
      Icon          string `json:"icon"`
      AutoShutdown  bool   `json:"auto_shutdown"`
  }
  ```
- **依存**: なし（純粋なファイル操作）。

**logger.go**
- **責務**: アプリケーション全体のログ出力管理。
- **機能**:
  - structured logging（推奨: zerolog または logrus）
  - settings.log_level に応じたログレベル設定（DEBUG/INFO/WARN/ERROR）
  - 標準出力 + オプションでファイル出力
  - context 情報の追加（モジュール名、リクエストID等）
- **実装例**:
  ```go
  func InitLogger(level string) *zerolog.Logger
  func LogError(err error, msg string)
  func LogInfo(msg string, fields map[string]interface{})
  ```
- **依存**: settings から log_level 取得。

---

## 通信フロー例

### 1. ユーザーがコンテナ起動ボタンをクリック

```
Discord
  → handlers.go: ボタンクリックを受信
  → commandChan に Command{Type: "start", ContainerID: "xxx"} を送信
  → main.go: commandChan から受信
  → docker.Start(ctx, "xxx") を呼び出し
  → state を更新
  → Discord に「起動しました」と返答
```

### 2. routine が状態変化を検知

```
routine.go
  → ticker で定期実行
  → docker.List() で最新情報取得
  → 前回のハッシュと比較 → 変化あり
  → statusUpdateChan に StatusUpdate 送信
  → main.go: statusUpdateChan から受信
  → discord.UpdateStatus() を呼び出し
  → Discord のメッセージを編集
```

### 3. 自動停止の判定

```
routine.go
  → コンテナ A がプレイヤー数ゼロ＆600秒経過を検知
  → settings で auto_shutdown が true か確認
  → commandChan に Command{Type: "auto_stop", ContainerID: "A"} 送信
  → main.go 経由で docker.Stop() 実行
  → state 更新 → Discord 通知
```

---

## エラーハンドリング方針

- **致命的エラー**（Docker 接続断、設定ファイル破損）: main.go で捕捉 → ログ出力 → 優雅なシャットダウン
- **回復可能エラー**（一時的な API エラー、コンテナ操作失敗）: リトライ → 失敗したら errorChan に送信 → Discord にエラー通知
- **ユーザーエラー**（無効な操作、権限不足）: Discord に ephemeral メッセージで返答

---

## 実装の優先順位

1. **utilities/settings.go** — 全体が依存
2. **utilities/logger.go** — デバッグに必須
3. **state/state.go** — データ管理の中心
4. **docker/container/status.go** — enum 定義
5. **docker/container/container.go** — コンテナ操作
6. **docker/docker.go** — Docker API ラッパー
7. **main.go** — mediator 実装
8. **routine/routine.go** — 監視ループ
9. **discord/discord.go** — Bot セッション
10. **discord/handlers.go + components.go** — UI とインタラクション

