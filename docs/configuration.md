# Configuration

ASK uses two configuration files: `ask.yaml` for project settings and `ask.lock` for version locking.

---

## ask.yaml

The main configuration file, created by `ask init`.

### Full Example

```yaml
version: "1.2"

# Installed skills (legacy format)
skills:
  - browser-use
  - computer-use

# Installed skills with metadata (new format)
skills_info:
  - name: browser-use
    description: Browser automation for AI agents
    url: https://github.com/browser-use/browser-use
  - name: computer-use
    description: Computer control capabilities
    url: https://github.com/anthropics/skills/tree/main/skills/computer-use

# Skill sources
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

### Fields

| Field | Description |
|-------|-------------|
| `version` | Configuration schema version |
| `skills` | List of installed skill names |
| `skills_info` | Detailed skill metadata |
| `repos` | List of skill sources |

---

## ask.lock

Version lock file for reproducible installations. **Do not edit manually.**

### Format

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

### Fields

| Field | Description |
|-------|-------------|
| `name` | Skill name |
| `url` | Source repository URL |
| `commit` | Exact Git commit hash |
| `version` | Version tag (if available) |
| `installed_at` | Installation timestamp (RFC3339) |

### Purpose

- **Reproducibility**: Same `ask.lock` = same skill versions
- **Team Sync**: Commit `ask.lock` to share exact versions
- **Update Detection**: Compare current vs locked commits

---

## Environment Variables

| Variable | Description |
|----------|-------------|
| `GITHUB_TOKEN` / `GH_TOKEN` | GitHub API token for higher rate limits |

### Example

```bash
export GITHUB_TOKEN=ghp_xxxxxxxxxxxx
ask search browser   # Uses authenticated API
```

---

## Default Sources

ASK automatically includes these sources, even if not in your `ask.yaml`:

| Name | Type | URL |
|------|------|-----|
| `featured` | registry | `yeasy/awesome-agent-skills/registry/index.json` |
| `anthropics` | dir | `anthropics/skills/skills` |
| `openai` | dir | `openai/skills/skills` |
| `composio` | dir | `ComposioHQ/awesome-claude-skills` |
| `vercel` | dir | `vercel-labs/agent-skills` |
| `openclaw` | dir | `openclaw/openclaw/skills` |

To add custom sources, see [Skill Sources](skill-sources.md).

---

## Best Practices

1. **Commit both files**: Add `ask.yaml` and `ask.lock` to version control
2. **Don't edit ask.lock**: Let ASK manage this file
3. **Review updates**: Check `ask outdated` before updating
4. **Pin versions**: Use `skill@version` for critical dependencies
