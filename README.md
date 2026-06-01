# CPA 自用版

这是基于 [router-for-me/CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI) 的自用构建，重点服务于 Codex/Responses 稳定性、多账号运行、NAS/Docker 部署和日常 CPA 管理。

本仓库保留上游 MIT 许可证和上游项目署名。它不是上游官方发布版，而是一个面向实际部署的 selfuse 分支，包含一些兼容性修复和运维 UI 增强。

## 本构建改动

### 1. Codex 上下文过长时交回客户端处理

当 Codex 上游以 `context_too_large` / `context_length_exceeded` 结束流式响应时，本构建不再在 CPA 中间层自行压缩历史、生成 `history.txt`、移除 reasoning 后继续重试。

当前处理方式：

- 保留上游式行为：上下文过长错误直接返回给客户端。
- 避免 CPA 把历史会话改写成新的请求后再次喂给模型。
- 避免长会话中出现“重新读取工作区、重新确认状态、重新规划”的重复循环。
- SSE 行破损修复仍然保留，二者是不同问题。

### 2. 加密 reasoning 上下文降级重试

部分 Codex/Responses 请求会携带 `input[*].encrypted_content`。当上游拒绝这段加密 reasoning 上下文时，原始 CPA 可能直接失败。

本构建会检测该类上游拒绝，移除无效的加密 reasoning 上下文，并重试一次。

效果：

- 减少 `encrypted_content` 被上游拒绝导致的失败。
- 提升对复用 Responses reasoning 上下文客户端的兼容性。
- 重试只针对明确的加密上下文拒绝场景。

### 3. reasoning / thinking 参数兼容

本构建增强了 thinking、reasoning effort 相关请求的兼容性。

效果：

- 客户端发送 reasoning 相关字段时更少直接失败。
- 更好处理 Codex/Responses 风格 reasoning payload。
- 当模型和客户端格式不一致时，行为更干净。

### 4. Codex 响应头超时

Codex 上游请求有时会在返回响应头前卡住。此时 CPA 无法判断上游是在排队、停滞，还是 HTTP/2 连接卡死。

本构建增加了只作用于响应头阶段的超时：

```yaml
codex-response-header-timeout-seconds: 180
```

行为：

- 只限制等待上游响应头的时间。
- 响应头到达后的流式正文不受该超时限制。
- 超时后进入现有重试和账号重选流程。
- 避免一次卡死的上游请求占住客户端很多分钟。

设置为负数可关闭该超时：

```yaml
codex-response-header-timeout-seconds: -1
```

也支持环境变量覆盖：

```bash
CPA_CODEX_RESPONSE_HEADER_TIMEOUT_SECONDS=180
```

### 5. 流式 keepalive 和启动阶段重试

建议配合以下流式保活和启动阶段重试配置使用：

```yaml
nonstream-keepalive-interval: 15
streaming:
  keepalive-seconds: 15
  bootstrap-retries: 1
```

行为：

- 下游客户端等待时可以收到 keepalive 事件。
- 首个流式 payload 前失败时可以安全重试。
- 降低慢上游启动导致的客户端误断开。

### 6. auth 文件模型测试控制

管理 UI 增强了 auth 文件测试能力。

新增控制：

- 单个 auth 文件的 `Test Model` 按钮。
- 当前页批量测试按钮。
- 每个账号的测试结果徽标：
  - 账号可用。
  - 账号不可用。

测试接口会固定选中的 auth 文件发送一个最小模型调用，并返回成功、失败和延迟。

### 7. Codex 超时的可视化配置项

管理页的可视化编辑器现在直接暴露：

```yaml
codex-response-header-timeout-seconds
```

这样常用的 Codex 响应头超时调优不需要切换到原始 YAML 编辑。

### 8. 国内 Docker 构建调整

本构建包含一些 Docker 相关调整，适合 Docker Hub 直连不稳定的环境。

实用说明：

- 镜像源可以切换为可访问的镜像站。
- build / pull 命令可以走 HTTP 代理。
- 可配合 HTTP 代理使用：

```text
http://<proxy-host>:<proxy-port>
```

### 9. CPA Manager 代理注入

如果部署里使用独立 CPAMC 容器，可以通过轻量的 manager proxy 向 CPAMC 注入本地 UI 增强，而不必重新构建 CPAMC 前端。

典型拓扑：

```text
浏览器
  -> cpa-manager-proxy :18317
  -> cpa-manager

客户端 / API
  -> cli-proxy-api :8317
```

该代理让 CPAMC 保持可用，同时加入本地自定义控制项。

## 推荐配置

示例运维配置：

```yaml
request-retry: 3
max-retry-credentials: 3
max-retry-interval: 30

routing:
  session-affinity: true

nonstream-keepalive-interval: 15
codex-response-header-timeout-seconds: 180

streaming:
  keepalive-seconds: 15
  bootstrap-retries: 1
```

## Docker Compose 使用

构建并启动：

```bash
docker compose up -d --build
```

如果环境需要代理：

```bash
export HTTP_PROXY=http://<proxy-host>:<proxy-port>
export HTTPS_PROXY=http://<proxy-host>:<proxy-port>
export NO_PROXY=localhost,127.0.0.1,<lan-host>
docker compose up -d --build
```

管理页和 API 端口取决于你的 compose 文件。参考部署中：

```text
CPA API:      http://<host>:8317
CPAMC 代理:   http://<host>:18317/management.html
```

## 版本规则

本仓库的自用发布版本固定使用 `selfuse` 后缀，例如：

```text
v7.1.35-selfuse.3
```

NAS 本地 Docker 镜像建议使用稳定标签：

```text
cli-proxy-api:v7.1.35-selfuse
```

这样日志、镜像、Release 和回滚点都能保持清晰。

## 安全说明

不要提交真实 auth 文件、refresh token、access token、management key 或 API key。

推荐作为运行态文件保留在仓库外或 `.gitignore` 中：

```text
auth-dir/
auths/
logs/
*.sqlite
*.db
config.yaml
```

公开 fork 或发布前，建议扫描敏感信息：

```bash
rg -n "github_pat_|refresh_token|access_token|id_token|sk-[A-Za-z0-9]|secret-key:" .
```

## 上游

本仓库基于：

- 上游项目：[router-for-me/CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI)
- 许可证：MIT，见 [LICENSE](LICENSE)

通用的小修复应尽量单独提交给上游。本仓库保留的本地运维改动，多数与 selfuse 部署和管理需求有关。

## 当前定位

这是自用构建，优先服务实际运行：

- Codex / Responses 兼容性。
- 多账号重试和账号重选。
- 可观察的账号测试。
- NAS / Docker 部署。
- 管理 UI 便利性。
- 历史上下文降级重试时避免旧命令和旧要求被重放。

除当前部署需求外，不额外承诺兼容性。
