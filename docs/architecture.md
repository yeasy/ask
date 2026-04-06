# ASK Architecture

This document describes the technical architecture of ASK (Agent Skills Kit), the package manager for AI Agent Skills.

## System Overview

ASK is designed as a lightweight, fast CLI tool built in Go that manages AI agent skills similar to how package managers like Homebrew or npm handle dependencies.

```mermaid
graph LR
    subgraph "User Interface"
        direction TB
        CLI[Terminal / CLI]
        GUI[Web UI / Desktop]
    end

    subgraph "ASK Core"
        direction TB
        Mgr[Skill Manager]
        Sec[Security Audit]
        Config[Config ask.yaml]
        Lock[Lock ask.lock]
    end

    subgraph "Cloud Ecosystem"
        GitHub[GitHub / Community]
        Official[Official Repos]
    end

    subgraph "Agent Environment"
        direction TB
        Project[.agent/skills/]
        Global[~/.ask/skills/]
        Agents{Agents}
    end

    CLI --> Mgr
    GUI --> Mgr
    
    Mgr <-->|Discover & Pull| GitHub
    Mgr <-->|Discover & Pull| Official
    
    Mgr -->|Scan| Sec
    Mgr <-->|Read/Write| Config
    Mgr -->|Write| Lock
    
    Mgr -->|Install| Project
    Mgr -->|Install| Global
    
    Project -.->|Load| Agents
    Global -.->|Load| Agents

    style Mgr fill:#4a9eff,color:white
    style Sec fill:#ff6b6b,color:white
    style Agents fill:#90ee90,color:black
```

## Core Components

### 1. CLI Layer (`cmd/`)

The command layer uses [Cobra](https://github.com/spf13/cobra) for CLI framework.

**Structure**:
```
cmd/
├── root.go          # Root command & config
├── init.go          # Project initialization
├── skill.go         # Skill parent command
├── search.go        # Skill search
├── install.go       # Skill installation
├── uninstall.go     # Skill removal
├── update.go        # Skill updates
├── outdated.go      # Check outdated skills
├── list.go          # List installed skills
├── info.go          # Skill information
├── create.go        # Create skill template
├── repo.go          # Repository management
└── completion.go    # Shell completion
```

**Command Flow**:
```mermaid
sequenceDiagram
    participant User
    participant CLI
    participant Config
    participant Internal

    User->>CLI: ask skill install browser-use
    CLI->>Config: Load ask.yaml
    Config-->>CLI: Configuration loaded
    CLI->>Internal: Execute install logic
    Internal->>GitHub: Clone repository
    GitHub-->>Internal: Repository downloaded
    Internal->>Config: Update ask.yaml & ask.lock
    Config-->>CLI: Config saved
    CLI-->>User: Installation complete
```

### 2. Internal Packages (`internal/`)

#### Config Management (`internal/config/`)

Handles configuration files and lock files.

**Key Files**:
- `config.go`: Main configuration logic
- `lock.go`: Version locking mechanism

**Data Structures**:
```go
type Config struct {
    Version    string
    Skills     []string
    SkillsInfo []SkillInfo
    Repos      []Repo
}

type LockFile struct {
    Version int
    Skills  []LockEntry
}
```

**Config Flow**:
```mermaid
graph LR
    A[Load ask.yaml] --> B{Exists?}
    B -->|Yes| C[Parse YAML]
    B -->|No| D[Create Default]
    C --> E[Merge Default Repos]
    D --> E
    E --> F[Return Config]
```

#### GitHub Integration (`internal/github/`)

Handles GitHub API interactions for skill discovery.

**Features**:
- Topic-based search (GitHub topics)
- Directory-based search (repository subdirectories)
- Result caching for performance

**API Interaction**:
```mermaid
sequenceDiagram
    participant ASK
    participant Cache
    participant GitHub

    ASK->>Cache: Check cached results
    alt Cache Hit (< 1hr)
        Cache-->>ASK: Return cached data
    else Cache Miss
        ASK->>GitHub: API Request (topic/dir search)
        GitHub-->>ASK: Repository list
        ASK->>Cache: Store results
        Cache-->>ASK: Return fresh data
    end
```

#### Git Operations (`internal/git/`)

Handles all Git-related operations.

**Key Functions**:
- `Clone()`: Standard git clone
- `SparseClone()`: Efficient subdirectory cloning
- `InstallSubdir()`: Install from repository subdirectory
- `GetLatestTag()`: Retrieve latest version tag
- `Checkout()`: Switch to specific version
- `GetCurrentCommit()`: Get commit SHA for locking

**Sparse Checkout Optimization**:
```mermaid
graph TD
    A[Start Install] --> B{Subdirectory?}
    B -->|Yes| C[Try Sparse Checkout]
    B -->|No| D[Full Clone]
    C --> E{Success?}
    E -->|Yes| F[Copy Subdirectory]
    E -->|No| G[Fallback to Full Clone]
    G --> F
    D --> H[Copy Entire Repo]
    F --> I[Installation Complete]
    H --> I
```

**Why Sparse Checkout?**
- **Speed**: Only downloads needed files
- **Disk Space**: Smaller footprint
- **Bandwidth**: Reduced network usage

For monorepos like `anthropics/skills`, this is 10-100x faster than full clone.

#### Skill Parsing (`internal/skill/`)

Parses `SKILL.md` files for metadata.

**SKILL.md Format**:
```yaml
---
name: browser-use
description: Browser automation for AI agents
version: 1.0.0
author: browser-use
tags:
  - browser
  - automation
dependencies:
  - playwright
---

# Browser Use

Detailed skill documentation...
```

**Parsing Flow**:
```mermaid
graph TB
    A[Read SKILL.md] --> B{Has Frontmatter?}
    B -->|Yes| C[Parse YAML Frontmatter]
    B -->|No| D[Extract from Content]
    C --> E[Return SkillMeta]
    D --> F[Parse Title & Description]
    F --> E
```

#### Dependency Resolution (`internal/deps/`)

Resolves skill dependencies in topological order.

**Dependency Graph**:
```mermaid
graph TD
    A[Main Skill] --> B[Dep 1]
    A --> C[Dep 2]
    B --> D[Dep 3]
    C --> D
    
    style A fill:#4a9eff
    style D fill:#90ee90
```

**Installation Order**: Dep 3 → Dep 1 → Dep 2 → Main Skill

**Circular Dependency Detection**:
```mermaid
graph LR
    A[Skill A] --> B[Skill B]
    B --> C[Skill C]
    C --> A
    
    style A fill:#ff6b6b
    style B fill:#ff6b6b
    style C fill:#ff6b6b
```

❌ This is detected and rejected to prevent infinite loops.

#### UI Components (`internal/ui/`)

Progress bars and spinners for user feedback.

**Components**:
- **Progress Bar**: For operations with known total (e.g., parallel searches)
- **Spinner**: For indeterminate operations (e.g., git clone)

#### Caching (`internal/cache/`)

Time-based caching for search results.

**Cache Strategy**:
- **TTL**: 1 hour (configurable)
- **Storage**: In-memory (could be persisted)
- **Key**: Search query hash
- **Invalidation**: Time-based expiry

#### Server (`internal/server/`)

Embedded HTTP server for the Web UI and Desktop application.

**Structure**:
- `server.go`: Server lifecycle and routing
- `handlers_skill.go`: Skill management API
- `handlers_repo.go`: Repository management API
- `handlers_system.go`: System configuration API

#### Service (`internal/service/`)

Background process management for the server.

**Features**:
- PID file management
- Process status checking
- Service lifecycle control

## Data Flow

### Skill Installation Flow

```mermaid
sequenceDiagram
    participant U as User
    participant C as CLI
    participant G as GitHub
    participant Git as Git Ops
    participant FS as File System
    participant Cfg as Config

    U->>C: ask skill install browser-use
    C->>Cfg: Load config
    C->>G: Resolve skill source
    G-->>C: Repository URL
    C->>Git: Clone repository
    Git->>FS: Download to .agent/skills/
    Git-->>C: Clone complete
    C->>FS: Parse SKILL.md
    FS-->>C: Skill metadata
    C->>Cfg: Update ask.yaml
    C->>Cfg: Add to ask.lock
    Cfg-->>C: Saved
    C-->>U: Installation complete
```

### Skill Search Flow

```mermaid
graph TB
    A[User Query] --> B[Load Config]
    B --> C[Get Repo List]
    C --> D[Parallel Search]
    D --> E1[Search Topic Repos]
    D --> E2[Search Dir Repos]
    E1 --> F[GitHub API: Topics]
    E2 --> G[GitHub API: Contents]
    F --> H[Combine Results]
    G --> H
    H --> I[Filter by Keyword]
    I --> J[Mark Installed]
    J --> K[Display Table]
```

## Skill Sources

### Source Types

```mermaid
graph TB
    Sources[Skill Sources]
    Sources --> Topic[Topic-based]
    Sources --> Dir[Directory-based]
    
    Topic --> T1[community:<br/>agent-skill topic]
    
    Dir --> D1[anthropics:<br/>skills/]
    Dir --> D2[scientific:<br/>claude-scientific-skills]
    Dir --> D3[superpowers:<br/>superpowers/skills]
    Dir --> D4[openai:<br/>skills/]
    Dir --> D5[matlab:<br/>skills/]
    
    style Topic fill:#ffd93d
    style Dir fill:#90ee90
```

### Source Discovery

**Topic-based** (GitHub Topics API):
```
https://api.github.com/search/repositories?q=topic:agent-skill+<keyword>
```

**Directory-based** (GitHub Contents API):
```
https://api.github.com/repos/<owner>/<repo>/contents/<path>
```

## File Structure

### Project Layout

```
my-agent-project/
├── ask.yaml              # Configuration manifest
├── ask.lock              # Version lock file
├── main.py               # Your agent code
└── .agent/
    └── skills/           # Installed skills
        ├── browser-use/
        │   ├── SKILL.md
        │   ├── scripts/
        │   └── references/
        └── web-surfer/
            ├── SKILL.md
            └── ...
```

**Agent-Specific Paths:**
- **Claude**: `.claude/skills/`
- **Cursor**: `.cursor/skills/`
- **Codex**: `.codex/skills/`

### ASK Installation

```
/usr/local/bin/
└── ask                   # Single binary (Go compiled)

~/.cache/ask/             # Optional cache directory
└── search-cache.db       # Search result cache
```

## Performance Optimizations

### 1. Parallel Search

Multiple repository sources are searched concurrently using goroutines:

```go
results := make(chan searchResult, len(repos))
for _, repo := range repos {
    go func(r Repo) {
        // Search this repo
        results <- searchRepo(r)
    }(repo)
}
```

**Performance Impact**: 5-10x faster than sequential search.

### 2. Sparse Checkout

Only download required subdirectories:

```bash
git clone --filter=blob:none --no-checkout --depth 1 <url>
git sparse-checkout init --cone
git sparse-checkout set <subdir>
git checkout
```

**Performance Impact**: 10-100x faster for monorepos.

### 3. Caching

Search results cached for 1 hour:
- Reduces GitHub API calls
- Faster repeated searches
- Prevents rate limiting

### 4. Single Binary

Go compiles to a single static binary:
- No runtime dependencies
- Fast startup time
- Easy distribution

## Security Considerations

### Trust Model

```mermaid
graph TB
    A[User Trusts] --> B[Repository Source]
    B --> C{Verified?}
    C -->|Official| D[anthropics, openai, MCP]
    C -->|Community| E[GitHub Topics]
    D --> F[Higher Trust]
    E --> G[User Verification Needed]
```

**Security Practices**:
1. **Read SKILL.md** before installation
2. **Review scripts/** directory contents
3. **Check repository stars/activity**
4. **Use version locking** for reproducibility
5. **Audit dependencies**

### Version Locking

`ask.lock` ensures reproducible installations:

```yaml
version: 1
skills:
  - name: browser-use
    url: https://github.com/browser-use/browser-use
    commit: abc123def456789...
    version: v1.2.0
    installed_at: 2026-01-15T08:00:00Z
```

This allows:
- **Exact reproduction** across environments
- **Rollback** to previous versions
- **Audit trail** of what was installed when

## Extension Points

### Custom Repositories

Users can add custom sources:

```yaml
repos:
  - name: my-team
    type: dir
    url: my-org/internal-skills/skills
```

### Future Extensibility

- **Plugin System**: Custom installers for non-Git sources
- **Registry API**: Central skill registry
- **Skill Templates**: More templates beyond default
- **Hooks**: Pre/post install hooks
- **Validation**: Skill quality checks

## Testing Strategy

### Unit Tests

Each `internal/` package has comprehensive tests:
- `config_test.go`: Config loading/saving
- `git_test.go`: Git operations
- `skill_test.go`: SKILL.md parsing
- `deps_test.go`: Dependency resolution
- `ui_test.go`: Progress bars

### Integration Tests

Command-level tests in `cmd/cmd_test.go`:
- Test command execution
- Verify help text
- Check error handling

### CI/CD

GitHub Actions workflows:
- **Lint**: golangci-lint, go fmt, go vet
- **Test**: Multi-platform (Ubuntu, macOS), Go 1.25+
- **Release**: Automated releases with goreleaser

## Monitoring & Observability

### Future Enhancements

- **Usage Analytics** (opt-in): Most popular skills
- **Error Reporting**: Crash reporting with consent
- **Performance Metrics**: Installation time tracking

---

## Technical Decisions

### Why Go?

1. **Single Binary**: Easy distribution
2. **Fast**: Compiled language
3. **Concurrency**: Goroutines for parallel operations
4. **Cross-platform**: Works on macOS, Linux, Windows
5. **Rich Ecosystem**: Great libraries (Cobra, progressbar)

### Why GitHub API?

1. **Ubiquitous**: Most skills already on GitHub
2. **Free tier**: Sufficient for most users
3. **Well-documented**: Stable API
4. **Version control**: Built-in versioning

### Why SKILL.md?

1. **Human-readable**: Easy to review
2. **Markdown**: Familiar format
3. **YAML frontmatter**: Structured metadata
4. **Extensible**: Can add more fields

---

## Performance Benchmarks

### Search Performance

| Repos | Sequential | Parallel | Speedup |
|-------|-----------|----------|---------|
| 1     | 1.2s      | 1.2s     | 1.0x    |
| 3     | 3.5s      | 1.4s     | 2.5x    |
| 6     | 7.1s      | 1.5s     | 4.7x    |

### Installation Performance

| Method | Time | Size |
|--------|------|------|
| Full clone (anthropics/skills) | 12.3s | 45 MB |
| Sparse checkout (single skill) | 1.1s | 2 MB |
| **Speedup** | **11x** | **22x** |

---

For more details, see:
- [Configuration Guide](configuration.md)
- [SKILL.md Format](skill-format.md)
- [Development Guide](../CONTRIBUTING.md)
