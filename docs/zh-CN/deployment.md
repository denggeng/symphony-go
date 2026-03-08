# 部署说明

`symphony-go` 优先面向自托管。推荐的初始部署路径为：

1. 为 UI 和 JSON API 启用 HTTP Basic Auth
2. 保持 `/healthz` 对容器或负载均衡探针开放
3. 在 Jira 模式下使用 `tracker.webhook_secret` 保护 webhook
4. 将工作区与任务数据挂载到持久化存储
5. 通过 Docker Compose 或你自己的反向代理部署服务

## Basic Auth

设置以下两个环境变量即可启用 HTTP Basic Auth：

- `SYMPHONY_SERVER_AUTH_USERNAME`
- `SYMPHONY_SERVER_AUTH_PASSWORD`

启用后，Dashboard、历史页面、SSE 流与 JSON API 都需要 Basic Auth。

`/healthz` 仍然保持匿名访问，以便容器健康检查继续工作。

`/api/v1/webhooks/jira` 接受以下任意方式：

- 有效的 Basic Auth 凭据；或
- 有效的 `tracker.webhook_secret`

## Docker Compose

仓库自带：

- `Dockerfile` — 构建 `symphonyd`
- `compose.yaml` — 本地自托管运行脚手架
- `docker/entrypoint.sh` — 启动时检查 `WORKFLOW.md`、工作区根目录与 `codex`

### 快速开始

```bash
cp .env.example .env
# 填写 SOURCE_REPO_URL、SOURCE_REPO_REF，以及所选 tracker 的相关变量

docker compose build
docker compose up -d
```

### 容器中的 Codex

镜像会构建 `symphonyd`，但你的环境仍需要一个可用的 `codex` CLI。

你可以选择：

- 在 `.env` 中设置 `CODEX_INSTALL_COMMAND`，让 Docker 构建时安装 `codex`
- 使用一个已经内置 `codex` 的基础镜像
- 在 `WORKFLOW.md` 中覆盖 `codex.command`，使用自定义运行时路径

### 持久化数据

当前 Compose 配置会把工作区持久化到命名卷：

- `symphony-workspaces`

如果你在生产中使用本地 Markdown 任务模式，也应持久化或绑定挂载这些目录：

- `local.inbox_dir`
- `local.state_dir`
- `local.archive_dir`
- `local.results_dir`

如果你希望把工作区直接挂载到宿主机目录，请把 `compose.yaml` 中的 volume 映射改成 bind mount。
