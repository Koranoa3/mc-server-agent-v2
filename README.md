# mc-server-agent (minimal)

This repository contains a minimal Go-based agent that:

- Loads environment variables from `.env` at startup.
- Reads and writes `settings.json` located in `/data/settings.json` inside the container.

Quick start with docker-compose (builds image then runs):

```bash
docker compose up --build
```

Notes:

- The `docker-compose.yml` mounts the repository `settings.json` into the container at `/data/settings.json`.
- `.env` is used as an `env_file` for docker-compose so sensitive values (like `DISCORD_TOKEN`) are not baked into the image.
- If you want to run locally without Docker, ensure you have Go 1.20+ and run:

```bash
go build -v -o mc-agent ./app
./mc-agent
```

Security: make sure you keep `.env` secret and do not commit it to git. `.dockerignore` already excludes `.env` and `settings.json` from build context to avoid embedding secrets.
# mc-server-agent (Go) — Docker-ready skeleton

このリポジトリは、Dockerコンテナ上で稼働する最小限のGo製Discordアプリケーションの骨組みです。起動時に`.env`を読み、`settings.json`の読み書きができることを確認するサンプルを提供します。

主なファイル
- `main.go` - `.env`を読み、`settings.json`を読み書きする簡単なプログラム。
 - `app/main.go` - `.env`を読み、`settings.json`を読み書きする簡単なプログラム。
- `Dockerfile` - マルチステージビルドでバイナリを作成し、ランタイムイメージを生成します。

ビルドと実行例

1. settings.json を永続化したい場合はホストに置き、コンテナの `/data/settings.json` にマウントします。
2. `.env` をコンテナに渡すには `--env-file` を使います。

例: ローカルでビルドして実行する

```bash
# イメージをビルド
docker build -t mc-agent:local .

# コンテナを実行（ホストの settings.json を /data にマウント）
docker run --rm -it \
  --env-file .env \
  -v "$PWD/settings.json:/data/settings.json" \
  mc-agent:local
```

動作確認
- コンテナの起動後、`settings.json` に `last_started` キーが追加/更新されます。

注意
- `settings.json` をコンテナ内で読み書き可能にするため、ホスト側のファイルパーミッションやボリュームマウント先を適切に設定してください。

Permissions / パーミッションについて

もしコンテナ起動後に `settings.json` に書き込みできず「permission denied」になる場合、原因はホスト上のファイル所有者（UID/GID）とコンテナ内の非 root ユーザーが一致しないためです。対処法は主に二つあります。

1) コンテナをホストと同じ UID:GID で動かす（推奨）

```bash
export UID=$(id -u)
export GID=$(id -g)
docker compose up --build
```

`docker-compose.yml` は変数 `${UID}` / `${GID}` を参照するようになっているため、シェルで環境変数をセットしてから起動するとコンテナ内のプロセスがホストのユーザー権限で動作します。あるいは `.env` に `UID=` と `GID=` を追加しても構いません。

2) ホストのファイルパーミッションを緩める（簡易）

```bash
chmod 666 settings.json
```

この方法は簡単ですがファイルが誰でも書き込めるようになるため、セキュリティ上のリスクがあります。可能であれば 1) の方法を使ってください。
