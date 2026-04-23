# ASK (Agent Skills Kit) - 产品规格说明书

## 项目概述

ASK 是一个用于管理 AI Agent 技能的命令行工具，类似于 Homebrew 管理 macOS 软件包。

### 核心价值
- 让 AI Agent 开发者能够快速发现、安装和管理技能
- 提供标准化的技能格式和元数据规范
- 支持多数据源的技能搜索

---

## 功能需求

### 已实现功能 ✅

| 命令 | 功能 | 状态 |
|------|------|------|
| `ask init` | 初始化项目，创建 `ask.yaml` | ✅ |
| **技能管理 (ask skill)** | | |
| `ask skill search <keyword>` | 搜索技能（显示来源、标记已安装） | ✅ |
| `ask skill install <skill>` | 安装技能（记录到 ask.lock） | ✅ |
| `ask skill install skill@v1.0` | 版本锁定安装 | ✅ |
| `ask skill uninstall <skill>` | 卸载技能 | ✅ |
| `ask skill list` | 列出已安装技能 | ✅ |
| `ask skill info <skill>` | 显示技能详情 | ✅ |
| `ask skill update [skill]` | 更新技能到最新版本 | ✅ |
| `ask skill outdated` | 检查可更新的技能 | ✅ |
| `ask skill create <name>` | 创建技能模板 | ✅ |
| **仓库管理 (ask repo)** | | |
| `ask repo add <NAME\|URL>` | 添加技能仓库来源 | ✅ |
| `ask repo list` | 列出所有来源 | ✅ |
| `ask repo remove <name>` | 移除来源 | ✅ |
| `ask completion <shell>` | 生成shell补全脚本 | ✅ |

### 待实现功能 ⏳

v0.4.0 已实现:
- [x] 性能基准测试 (`ask benchmark`)
- [x] 离线模式 (`--offline`)

v0.5.0 已实现:
- [x] 全局安装支持 (`--global` / `-g`)
  - 全局技能目录: `~/.ask/skills/`
  - 全局配置文件: `~/.ask/config.yaml`
  - 全局锁定文件: `~/.ask/ask.lock`
- [x] 多 Agent 支持 (`--agent` / `-a`)
  - 支持将技能安装到特定 Agent 目录
  - 支持的 Agent: Amp, Antigravity, Claude, ClawdBot, CodeBuddy, Codex, Copilot, Cursor, Droid, Gemini, Goose, Kilo, Kiro, Neovate, OpenClaw, OpenCode, Roo, Trae, Windsurf
  - 示例: `ask skill install <skill> --agent claude --agent cursor`

### v0.6.0 已实现:
- [x] 仓库详情查看 (`ask repo list [name]`)
- [x] 完整的测试覆盖和 CI/CD 流程
- [x] `GH_PAT` 支持发布到 Homebrew Tap
- [x] 自动提取 Git 作者信息

### v1.0.0 已实现:
- [x] 安全检查 (`ask check`)
- [x] 熵值分析
- [x] 详细安全报告 (HTML/JSON)

### v1.1.0 已实现:
- [x] 批量扫描
- [x] 增强的 HTML 报告
- [x] 仓库同步优化

### v1.3.0 已实现:
- [x] Web UI 和桌面应用
- [x] Wails 集成
- [x] 可视化仪表盘

### v1.4.0 建议功能:
- [ ] 插件系统


---

## 技能来源规范

### 支持的来源类型

| 类型 | 说明 | 示例 |
|------|------|------|
| `topic` | GitHub 主题搜索 | `agent-skill` |
| `dir` | GitHub 仓库目录 | `anthropics/skills/skills` |
| `registry` | JSON 注册表索引 | `yeasy/awesome-agent-skills/registry/index.json` |

### 默认来源

```yaml
repos:
  - name: featured
    type: registry
    url: yeasy/awesome-agent-skills/registry/index.json
  - name: anthropics
    type: dir
    url: anthropics/skills/skills
  - name: openai
    type: dir
    url: openai/skills/skills
  - name: composio
    type: dir
    url: ComposioHQ/awesome-claude-skills
  - name: vercel
    type: dir
    url: vercel-labs/agent-skills
  - name: openclaw
    type: dir
    url: openclaw/openclaw/skills
```

### 可选来源

```yaml
repos:
  - name: community
    type: topic
    url: "agent-skill OR topic:agent-skills"
```

---

## SKILL.md 规范

每个技能应包含 `SKILL.md` 文件，支持 YAML frontmatter：

```yaml
---
name: browser-use
description: Browser automation for AI agents
version: 1.0.0
author: browser-use
dependencies:
  - playwright
tags:
  - browser
  - automation
---

# Browser Use

技能的详细说明...
```

---

## ask.yaml 规范

项目配置文件格式：

```yaml
version: "1.0"
skills:
  - browser-use      # 已安装技能列表
skills_info:         # 技能元数据
  - name: browser-use
    description: Browser automation
    url: https://github.com/browser-use/browser-use
repos:             # 可选：自定义来源
  - name: custom
    type: dir
    url: owner/repo/path
```

---

## ask.lock 规范

版本锁定文件，确保可复现安装：

```yaml
version: 1
skills:
  - name: browser-use
    url: https://github.com/browser-use/browser-use
    commit: abc123def456
    version: v1.0.0
    installed_at: 2026-01-15T08:00:00Z
```

---

## 技术架构

### 目录结构

```
ask/
├── cmd/                  # 命令实现
│   ├── root.go
│   ├── init.go
│   ├── skill.go          # 技能父命令
│   ├── search.go
│   ├── install.go
│   ├── uninstall.go
│   ├── update.go
│   ├── outdated.go
│   ├── list.go
│   ├── info.go
│   ├── create.go
│   └── repo.go
├── internal/
│   ├── config/           # 配置管理（含 lock.go）
│   ├── github/           # GitHub API 客户端
│   ├── git/              # Git 操作
│   ├── skill/            # SKILL.md 解析
│   ├── deps/             # 依赖解析
│   ├── server/           # HTTP 服务器
│   └── service/          # 进程管理
├── assets/               # 静态资源（logo等）
├── .github/workflows/    # CI/CD
├── Makefile
├── README.md
└── README_zh.md
```

### 性能优化

1. **并行搜索**: 使用 goroutines 并行扫描多个来源
2. **子目录安装**: 克隆到临时目录，复制子目录
3. **默认来源合并**: `LoadConfig` 自动合并新增默认来源

---

## 发布规范

### 版本号
遵循 [Semantic Versioning](https://semver.org/):
- MAJOR: 不兼容的 API 变更
- MINOR: 向后兼容的功能新增
- PATCH: 向后兼容的问题修复

### 发布流程
1. 更新 CHANGELOG.md
2. 创建 tag: `git tag v0.1.0`
3. 推送: `git push --tags`
4. GitHub Actions 自动运行 goreleaser

### Homebrew 安装
需要创建 `yeasy/homebrew-tap` 仓库，goreleaser 会自动生成 formula。

---

## 代码规范

### Go 代码风格
- 使用 `gofmt` 格式化
- 使用 `go vet` 检查
- 公共函数需要注释

### 测试要求
- 所有 internal 包需要测试文件
- 测试命令: `make test`
- CI 自动运行测试

### 提交规范
```
<type>: <description>

[optional body]
```

类型: feat, fix, docs, refactor, test, chore

---

## 待办事项

### 高优先级 (P1)
- [x] `ask update` 命令
- [x] 版本锁定 (`skill@v1.0`)
- [x] Shell 补全支持
- [x] Homebrew tap 仓库验证

### 中优先级 (P2)
- [x] Git sparse checkout 优化
- [x] 搜索结果缓存
- [x] 进度条显示
- [x] `ask create` 命令
- [x] 完整的测试覆盖

### 低优先级 (P3)
- [ ] 插件系统
- [ ] 自定义注册表 API

---

## 更新日志

| 日期 | 版本 | 变更 |
|------|------|------|
| 2026-01-16 | 0.6.1 | 修复发布工作流权限问题 |
| 2026-01-16 | 0.6.0 | 新增仓库详情查看 (`ask repo list [name]`)，完善测试和 CI/CD |
| 2026-01-16 | 0.5.0 | 新增多 Agent 支持 (`--agent`), 全局安装 (`--global`), 以及智能目录检测 |
| 2026-01-16 | 0.4.0 | 新增离线模式 (`--offline`) 和性能基准测试 (`ask benchmark`) |
| 2026-01-15 | 0.2.0 | CLI 重构：技能命令移至 `ask skill` 子命令；技能安装路径改为 `.agent/skills/`；新增 OpenAI 等默认仓库 |
| 2026-01-15 | 0.1.0 | 初始版本，基本功能实现 |
