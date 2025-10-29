# mc-server-agent — 要約 / Summary

軽量なGo製エージェントのテンプレート。起動時に`.env`を読み、コンテナ内の`/data/settings.json`を読み書きします。

使い方（簡潔）
- Docker: イメージをビルドして起動
    ```bash
    docker compose up --build
    ```
- ローカル (Go 1.20+)
    ```bash
    go build -v -o mc-agent ./app
    ./mc-agent
    ```

重要ポイント
- `docker-compose.yml` はホストの `settings.json` を `/data/settings.json` にマウントする想定。
- `.env` は `env_file` として使い、機密値はイメージに含めない。
- `.dockerignore` で `.env` と `settings.json` を除外済み。

権限トラブル対処
1) 推奨: コンテナをホストと同じ UID/GID で実行
     ```bash
     export UID=$(id -u)
     export GID=$(id -g)
     docker compose up --build
     ```
2) 簡易 (非推奨、セキュリティリスクあり): ← 採用
     ```bash
     chmod 666 settings.json
     ```

目的は起動確認と `settings.json` の読み書きテストです。  
セキュリティのため`.env`はコミットしないでください。
