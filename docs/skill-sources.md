# Skill Sources

ASK can search and install skills from multiple sources. This document explains how sources work and how to configure them.

---

## Default Sources

ASK comes with six pre-configured sources:

| Name | Type | URL | Description |
|------|------|-----|-------------|
| `featured` | registry | `yeasy/awesome-agent-skills/registry/index.json` | Featured skills registry |
| `anthropics` | dir | `anthropics/skills/skills` | Official Anthropic skills |
| `openai` | dir | `openai/skills/skills` | OpenAI official skills |
| `composio` | dir | `ComposioHQ/awesome-claude-skills` | Awesome Claude Skills |
| `vercel` | dir | `vercel-labs/agent-skills` | Vercel AI SDK skills |
| `openclaw` | dir | `openclaw/openclaw/skills` | OpenClaw skills |

---

## Source Types

### Topic Sources (`topic`)

Topic sources search GitHub for repositories with a specific topic tag.

```yaml
repos:
  - name: community
    type: topic
    url: agent-skill   # The GitHub topic to search
```

**Pros:**
- Discovers community-maintained skills automatically
- New skills appear as they're published

**Cons:**
- Search results depend on GitHub API limits
- Quality varies across community projects

### Registry Sources (`registry`)

Registry sources point to a JSON index file that lists curated skills with metadata.

```yaml
repos:
  - name: featured
    type: registry
    url: yeasy/awesome-agent-skills/registry/index.json
```

**Pros:**
- Centrally curated with rich metadata (descriptions, tags, scores)
- Fast lookups from a single index file
- Ideal for featured or recommended skill lists

**Cons:**
- Requires maintaining the registry index file
- Skills must be explicitly added to the registry

### Directory Sources (`dir`)

Directory sources point to a specific path within a GitHub repository.

```yaml
repos:
  - name: anthropics
    type: dir
    url: anthropics/skills/skills   # owner/repo/path
```

**Pros:**
- Curated, consistent quality
- Faster to search (no API queries)
- Works well for organization-managed skills

**Cons:**
- New skills require repository updates

---

## Adding Custom Sources

Edit your `ask.yaml` to add custom sources:

```yaml
version: "1.2"
skills:
  - browser-use
repos:
  # Add your custom sources here
  - name: my-team
    type: dir
    url: my-org/agent-skills/skills
  
  - name: awesome-skills
    type: topic
    url: awesome-agent-skill
```

```bash
ask repo add my-org/agent-skills/skills
```

---

## Viewing Sources

List all configured sources:

```bash
ask repo list
```

List skills available in a source:

```bash
ask repo list <source-name>
```

---

## Removing Sources

Remove a source by name:

```bash
ask repo remove my-team
```

> **Note:** Default sources are always available and will be re-added on next `LoadConfig()` call.

---

## Search Priority

When you run `ask search`, ASK queries all sources in parallel and merges results. If the same skill exists in multiple sources, both entries are shown with their source indicated.

---

## GitHub API Rate Limits

Topic-based searches use the GitHub Search API, which has rate limits:
- **Unauthenticated**: 10 requests/minute
- **Authenticated**: 30 requests/minute

To increase limits, set a GitHub token:

```bash
export GITHUB_TOKEN=your_token_here
```

Directory-based sources use the Contents API which has higher limits.
