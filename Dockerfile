# syntax=docker/dockerfile:1.7
FROM golang:1.24-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/symphonyd ./cmd/symphonyd

FROM debian:bookworm-slim
ARG CODEX_INSTALL_COMMAND=""
RUN apt-get update \
    && apt-get install -y --no-install-recommends bash ca-certificates curl git \
    && rm -rf /var/lib/apt/lists/* \
    && if [ -n "$CODEX_INSTALL_COMMAND" ]; then sh -lc "$CODEX_INSTALL_COMMAND"; fi
WORKDIR /app
COPY --from=build /out/symphonyd /usr/local/bin/symphonyd
COPY docker/entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh
ENV SYMPHONY_WORKFLOW_PATH=/app/WORKFLOW.md
EXPOSE 8080
ENTRYPOINT ["docker-entrypoint.sh"]
CMD ["symphonyd", "-workflow", "/app/WORKFLOW.md", "-log-level", "info"]
