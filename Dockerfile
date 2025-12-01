FROM debian:12-slim

# CA証明書をインストール
RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates && \
    rm -rf /var/lib/apt/lists/*

# アプリケーション用ユーザーとグループを作成
RUN groupadd -r app && useradd -r -g app app

# ビルド済みバイナリをコピー
COPY mc-agent /usr/local/bin/mc-agent
RUN chmod +x /usr/local/bin/mc-agent

WORKDIR /data
RUN chown app:app /data || true
USER app
ENV SETTINGS_PATH=/data/settings.json
ENTRYPOINT ["/usr/local/bin/mc-agent"]

