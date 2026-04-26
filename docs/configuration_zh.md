# 配置指南

ASK 使用两个配置文件：`ask.yaml` 用于项目设置，`ask.lock` 用于版本锁定。

---

## ask.yaml

主配置文件，由 `ask init` 创建。

### 完整示例

```yaml
version: "1.2"

# 已安装的技能 (旧格式)
skills:
  - browser-use
  - computer-use

# 已安装的技能及其元数据 (新格式)
skills_info:
  - name: browser-use
    description: Browser automation for AI agents
    url: https://github.com/browser-use/browser-use
  - name: computer-use
    description: Computer control capabilities
    url: https://github.com/anthropics/skills/tree/main/skills/computer-use

# 技能源
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

### 字段说明

| 字段 | 描述 |
|-------|-------------|
| `version` | 配置架构版本 |
| `skills` | 已安装技能名称列表 |
| `skills_info` | 详细的技能元数据 |
| `repos` | 技能源列表 |

---

## ask.lock

用于可重复安装的版本锁定文件。**请勿手动编辑。**

### 格式

```yaml
version: 1
skills:
  - name: browser-use
    url: https://github.com/browser-use/browser-use
    commit: abc123def456789
    version: v1.2.0
    installed_at: 2026-01-15T08:00:00Z
  - name: computer-use
    url: https://github.com/anthropics/skills
    commit: def789abc123456
    version: ""
    installed_at: 2026-01-15T08:30:00Z
```

### 字段说明

| 字段 | 描述 |
|-------|-------------|
| `name` | 技能名称 |
| `url` | 来源仓库 URL |
| `commit` | 确切的 Git 提交哈希 |
| `version` | 版本标签 (如果有) |
| `installed_at` | 安装时间戳 (RFC3339) |

### 用途

- **可重复性**: 相同的 `ask.lock` = 相同的技能版本
- **团队同步**: 提交 `ask.lock` 以共享确切版本
- **更新检测**: 比较当前版本与锁定版本的提交

---

## 环境变量

| 变量 | 描述 |
|----------|-------------|
| `GITHUB_TOKEN` / `GH_TOKEN` | 用于提高速率限制的 GitHub API 令牌 |
| `ASK_GITHUB_TOKEN` | ASK 专用 GitHub 令牌（优先级高于 `GITHUB_TOKEN`） |

### 示例

```bash
export GITHUB_TOKEN=ghp_xxxxxxxxxxxx
ask search browser   # 使用经过身份验证的 API
```

---

## 默认源

ASK 自动包含这些源，即使它们不在您的 `ask.yaml` 中：

| 名称 | 类型 | URL |
|------|------|-----|
| `featured` | registry | `yeasy/awesome-agent-skills/registry/index.json` |
| `anthropics` | dir | `anthropics/skills/skills` |
| `openai` | dir | `openai/skills/skills` |
| `composio` | dir | `ComposioHQ/awesome-claude-skills` |
| `vercel` | dir | `vercel-labs/agent-skills` |
| `openclaw` | dir | `openclaw/openclaw/skills` |

要添加自定义源，请参阅 [技能源](skill-sources_zh.md)。

---

## 最佳实践

1. **提交两个文件**: 将 `ask.yaml` 和 `ask.lock` 添加到版本控制
2. **不要编辑 ask.lock**: 让 ASK 管理此文件
3. **审查更新**: 在更新之前检查 `ask outdated`
4. **锁定版本**: 对关键依赖项使用 `skill@version`
