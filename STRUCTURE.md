# アプリケーションの構造

## ディレクトリ構成

```

app/
	main.go
	internal/
		discord/
			discord.go
			menu.go
			formatter/
				status_message.go
				container_list.go
		docker/
			docker.go
			container/
				container.go
				working_status.go
				player_list.go
		routine/
			routine.go
		utilities/
			whitelist.go
			settings.go
			env.go
	go.mod
	go.sum
settings.json
.env

```

## goモジュール構成

**main.go**
- 本体。
- discordオブジェクトの作成、routineスレッドの作成、メインループを担う。

### discord

**discord.go**
- スラッシュコマンドハンドラ(active)
- ステータス更新メソッド(passive)

**menu.go**
- passive -> active
- 命令を受けて、ボタンを生成。
- settingsに登録されたコンテナが複数ある場合、まずはコンテナ選択ボタンを生成。「すべてのコンテナの状態を表示」ボタンも配置
- コンテナが選択されたら、そのコンテナに対する操作ボタンを生成（状態に応じて。起動/停止）
- ボタンのメイン関数はdiscord.goが処理するが、その他の設定などはmenu.goで行う

#### discord/formatter

**status_message.go**
- 受け取ったデータを、botのdiscord上に表示するステータスメッセージ用に整形する。
**container_list.go**
- 受け取ったデータを、botがユーザーに返答する用に整形する。

### docker

**docker.go**
- passive
- settingsに登録されたコンテナを取得(Allから。稼働中/停止中/存在しないものもある)
- コンテナはcontainer.goが提供するオブジェクトとして返す

#### docker/container

**container.go**
- containerオブジェクトの定義
- go-dockerclientをラップし、各コンテナの操作を行う
- 開始/停止、各コンテナの稼働状況、プレイヤーリスト、ホワイトリストの取得/更新を行う
- containerオブジェクト内に状態を保持し、ハッシュ化して変更検知に使う

**working_status.go**
- 各コンテナの稼働状況を定義するenum
- 稼働中/起動中/停止中/存在しない/不明
- コンテナの稼働状況を判定するロジックを提供する
- マインクラフト特有の「起動中」などのステータスはcontainer inspectからHealthcheck関係を取得する

**player_list.go**
- マインクラフトのプレイヤーリストを取得するロジックを提供する
- RCONあるいはcontainer exec>listコマンドで取得する
- プレイヤーリストのパースも行う

### routine

**routine.go**
- active
- メインと別スレッドで稼働。
- docker.goの"定期的なコンテナ稼働状況チェック"を発火し、discord.goで"ステータス更新"に横流しする
- 各コンテナの稼働/非稼働を監視し、一定時間以上稼働していないコンテナは、各コンテナ設定に基づき自動停止判断➡停止命令を発火する。

### utilities

**settings.go**
  - SETTINGS_PATH (> settings.json)を読み、goが読みやすい形式にする
  - atomic 書き込み（tmp -> rename）、ロック（flock）を使う。並列プロセスからの競合対策が必要。

**env.go**
  - 環境変数の読み取り

**whitelist.go**
- WHITELIST_PATH (> whitelist.json)を読み、ホワイトリストの管理を行う

