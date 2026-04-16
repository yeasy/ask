# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.9.7] - 2026-04-15

### Fixed
- Fix file descriptor leak for `/dev/null` in `service start`.
- Replace fixed 1s sleep with polling loop for graceful service shutdown.
- Fix debounce timer race condition in file watcher.
- Skip 15 common non-skill directories in all WalkDir callbacks.
- Pin chart.js CDN to v4.5.1 with explicit UMD bundle path.
- Cache parsed HTML report template via `sync.Once`.
- Unify registry token resolution to include `ASK_GITHUB_TOKEN`.
- Reuse shared HTTP client for registry requests.
- Replace `strings.Split` with `strings.Count` for zero-alloc depth calc in scan.
- Deduplicate directories returned by `GetAllAgentSkillsDirs`.

## [1.9.6] - 2026-04-13

### Fixed
- Reuse HTTP clients for connection pooling across GitHub API calls.
- Add `ASK_GITHUB_TOKEN` support in repo validation functions.
- Validate SSH host in `ParseRepoURL` to reject non-GitHub hosts.
- Fix loop variable pointer bug in repo name matching.
- Remove duplicate `--global` flag registration across subcommands.
- Remove dead path-separator check after `filepath.Base` in uninstall.
- Add error logging for search, sync, and directory walk operations.
- Redirect background service stdin to `/dev/null`.
- Write progress bar completion newline to stderr consistently.
- Fix uninstall documentation to clarify `--all` flag behavior.
- Expand architecture docs, llm.txt, and command reference.

## [1.9.5] - 2026-04-06

### Fixed
- Consolidated duplicate `atomicWriteFile` into `filesystem.AtomicWriteFile` with fsync.
- Fixed variable shadowing of `errors` builtin in search command.
- Added cache file size limit via `io.LimitReader` to prevent OOM.
- Removed unused cache import in benchmark.
- Fixed outdated Go version and hardcoded agent list in help text.

## [1.9.4] - 2026-04-04

### Fixed
- **Security**: Case-insensitive matching in `IsSourceAllowed` to prevent bypass via mixed-case URLs.
- **Security**: Path traversal rejection in registry URL parsing.
- **Security**: Extended shell metacharacter blocklist, added `Vary: Origin` and `X-Content-Type-Options: nosniff` headers.
- **Security**: Validate paths before state changes in config update handler with rollback on failure.
- **Bug**: Send auth token in GitHub API requests for repo content fetching.
- **Bug**: Fix shared context timeout across fallback git commands in outdated check.
- **Bug**: Add timeouts to git exec commands in publish and template operations.
- **Bug**: Fix lock ordering in file watcher debounce timer cleanup.
- **Bug**: Fix flaky cache test timing under race detector.
- **Docs**: Fix incorrect brew tap name, debug command, and missing command flags documentation.

## [1.9.3] - 2026-04-01

### Fixed
- **Security**: Global config paths now return errors instead of falling back to CWD when HOME is unavailable.
- **Security**: Skill-bundled `.askcheck.yaml` can no longer disable CRITICAL security rules.
- **Security**: Fixed `IsPathIgnored` substring matching bypass (e.g., `vendor/**` no longer matches `vendor-tools`).
- **Security**: Added `json:"-"` tag to `Repo.Token` to prevent accidental JSON serialization.
- **Security**: `sanitizeAndRestrictPath` now resolves symlinks to prevent path bypass.
- **Security**: `InstallSubdir` validates subdirectory path before both sparse and fallback clone paths.
- **Security**: Tightened URL scheme checks from `HasPrefix("http")` to `HasPrefix("http://")` / `HasPrefix("https://")`.
- **Security**: `OpenBrowser` rejects URLs with shell metacharacters on Windows.
- **Security**: HTTP path validation errors logged server-side instead of forwarded to clients.
- **Security**: Reduced `NET-IP-ADDR` false positives on version strings and loopback addresses.
- **Security**: Custom rule regex compilation errors now warn to stderr instead of being silently discarded.
- **Security**: `cloneForScore` now has a 5-minute timeout.
- **Docs**: Added documentation for 12 missing CLI commands in `docs/commands.md`.
- **Docs**: Synced README_zh.md with English README (Go install method, desktop app section).
- **Docs**: Corrected CHANGELOG version splitting for v1.9.1/v1.9.2.

## [1.9.2] - 2026-04-01

### Fixed
- **Security**: Added file size limits and `LimitReader` wrapping to `CopyFile`.
- **Security**: Added cleanup of partial destination files on copy errors.
- **Security**: HTTP handlers no longer leak internal error messages to clients.
- **Security**: Added path traversal rejection in dependency resolver.
- **Security**: Added git operation timeouts to prevent indefinite hangs.
- **Security**: JSON responses marshal to buffer first to avoid partial writes.
- **Testing**: Added symlink and non-regular file tests.
- **Docs**: Updated README and README_zh command tables with missing entries.

## [1.9.1] - 2026-03-30

### Fixed
- **Code Quality**: Cleaned up debug print statements and improved doc comments.
- **HTTP**: Fixed response body double `LimitReader` wrapping that could cause read failures.
- **Security**: Fixed TOCTOU race conditions in symlink checks using open-then-fstat pattern.

## [1.9.0] - 2026-03-29

### Changed
- **CI**: Updated GitHub Actions to latest versions and hardened workflow permissions.
- **Dependencies**: Upgraded to Go 1.25 and updated all dependencies.
- **Dependencies**: Updated `golang.org/x/crypto` and `golang.org/x/net` for security patches.
- **Dependencies**: Bumped `github.com/wailsapp/wails/v2` from 2.11.0 to 2.12.0.

### Fixed
- **Security**: Fixed path traversal vulnerabilities in `sync` command.
- **Security**: Fixed YAML template escape issues in skill template generation.
- **Install**: Fixed `lock-install` global flag not propagating correctly.
- **Install**: Fixed `uninstall` command not resolving skill paths properly.

## [1.8.1] - 2026-03-29

### Added
- **Testing**: Expanded test coverage across all major packages with comprehensive test suites.

### Fixed
- **Security**: Fixed XSS vulnerabilities and hardened HTTP server security.
- **Security**: Hardened skill scanning, validation, and trust scoring.
- **Security**: Hardened security in core packages (git, installer, config, filesystem, cache).
- **CLI**: Improved input validation and error messages across all commands.
- **Lint**: Fixed errcheck lint warnings.

## [1.8.0] - 2026-03-27

### Added
- **Scoring**: Enhanced skill trust scoring system with improved category weights and detection patterns.

### Fixed
- **Security**: Hardened security and fixed bugs across multiple packages.
- **Dependencies**: Updated dependencies and gitignore.

## [1.7.9] - 2026-03-14

### Fixed
- **Concurrency**: Added `sync.RWMutex` to protect global `searchCache` from race conditions during concurrent GitHub API access.
- **Performance**: Replaced O(n²) duplicate skill check in `install` restore with O(1) map-based lookup.
- **Security**: Added git ref validation in `Checkout()` to reject malformed references containing `..`, shell metacharacters, or leading `-`.
- **Security**: Added path traversal protection in `buildRepoURL` and `buildRepoName` to reject `..` and empty segments.
- **Reliability**: Replaced `io.ReadAtLeast` with `io.ReadAll(io.LimitReader(...))` for safer SKILL.md description reading.
- **Code Quality**: Consolidated hardcoded HTTP timeout values into named constants (`httpTimeoutDefault`, `httpTimeoutShort`).
- **UX**: Added OS signal handling (`Ctrl+C`) for graceful cancellation of `repo sync` operations.
- **Code Quality**: Removed redundant `splitLines`/`trimSpace` helper functions in favor of `strings` stdlib.

## [1.7.8] - 2026-03-09

### Added
- **Scoring**: Implemented `ask skill score` command for skill trust evaluation.
- **Agents**: Added OpenClaw agent support.

## [1.7.7] - 2026-03-09

### Changed
- **UX**: Refined wording and improved list output formatting.

### Added
- **Registry**: Implemented skill registry support.

## [1.7.6] - 2026-03-09

### Added
- **Install**: Implemented `ask lock-install` command for reproducible installations.
- **Watch**: Implemented file watch mode for skill development.
- **Config**: Added support for customized security rules.
- **CI**: Added GitHub Actions workflows.
- **Init**: Added interactive init workflow.

## [1.7.5] - 2026-03-08

### Fixed
- **Config**: Updated config tests for featured registry repo.
- **Template**: Fixed backtick escaping in skill template README.

### Added
- **Search**: Show popular skills overview when searching without keyword.
- **Install**: Show install sync summary with agent names.

## [1.7.4] - 2026-03-04

### Fixed
- **Security**: Fixed path traversal vulnerability where `"."` as skill name could delete entire skills directory.
- **Security**: Fixed CORS origin validation accepting `localhost.evil.com` by enforcing strict prefix matching.
- **Security**: Fixed argument injection in server handlers (`handleRepoAdd`, `handleSkillImport`, `handleRepoSync`) by rejecting inputs starting with `-`.
- **Security**: Added `validateSkillName` check in `handleSkillFiles` to prevent directory traversal via skill name parameter.
- **Security**: Fixed `limitRequestBody` passing `nil` ResponseWriter to `http.MaxBytesReader`, preventing proper connection signaling.
- **Crash**: Fixed nil pointer dereference in `SaveIndexWithStars` when `os.Stat` fails on a cached repo directory.
- **Crash**: Fixed nil pointer dereference in `installer.Install` when `cache.NewReposCache()` fails.
- **Crash**: Fixed nil pointer dereference in `uninstall` and `outdated` commands when lock file fails to parse.
- **Commands**: Fixed `update` and `outdated` commands ignoring skills stored in `SkillsInfo` (only checking legacy `Skills` list).
- **Windows**: Fixed `OpenBrowser` command injection via URLs containing `&` by adding empty title argument to `start`.
- **Windows**: Fixed `sanitizeRepoName` not sanitizing backslashes or `..`, enabling path traversal on Windows.
- **Entropy**: Fixed `CalculateEntropy` using byte length instead of rune count, producing incorrect values for multi-byte UTF-8 strings.
- **Template**: Fixed `CreateSkillTemplate` writing literal `{{.Name}}` in generated script instead of the actual skill name.
- **Config**: Fixed `GetSkillInfo` and `LockFile.GetEntry` returning pointer to loop variable copy instead of actual slice element.
- **Config**: Fixed `GetAgentSkillsDir` returning `("", nil)` for unknown agent types instead of a proper error.
- **Server**: Fixed `handleRepoSync` reporting success even when the sync command fails.
- **Server**: Added mutex protection for `os.Chdir` in `handleConfigUpdate` to prevent race conditions between concurrent requests.
- **Resource Leak**: Fixed background `repo sync` process in `search.go` not calling `cmd.Wait()`, leaking goroutines.
- **Resource Leak**: Fixed log file handle never closed in `service.go` after starting background service.
- **GitHub**: Fixed `fetchSkillDescription` using single `Read` call that may return partial data; now uses `io.ReadAtLeast`.
- **GitHub**: Fixed `truncate` function slicing by byte index, potentially producing invalid UTF-8.
- **SkillHub**: Fixed unbounded `io.ReadAll` in `Resolve` that could cause OOM; now limited to 10MB.
- **Files**: Fixed `CopyFile` and `git.copyFile` not checking close errors on destination file; now uses named return with deferred close.
- **Code**: Removed redundant duplicate `if len(errors) > 0` check in `search.go`.
- **Cache**: Fixed `GetReposCacheDir` silently returning relative path when `os.UserHomeDir()` fails.
- **Install**: Fixed `installer.Install` not validating agent type, silently using empty directory for unknown agents.

## [1.7.3] - 2026-03-04

### Fixed
- **Windows**: Fixed `service.IsRunning()` using unsupported `syscall.Signal(0)` on Windows; now uses platform-specific build tags with `OpenProcess`/`GetExitCodeProcess`.
- **Windows**: Fixed path separator mismatch in skill install arguments on Windows by normalizing to forward slashes.
- **Crash**: Fixed potential panic in `benchmark` command when no repos are configured.
- **Crash**: Fixed potential panic in `info` command when file name is empty.
- **Install**: Fixed confusing fetch fallback logic that could silently swallow errors or skip API-based fetch.
- **Init**: `ensureInitialized()` now correctly returns `false` when initialization fails.
- **Server**: Replaced fragile error string comparison with `errors.Is(err, http.ErrServerClosed)`.
- **Server**: Strengthened path traversal defense using `filepath.Rel` verification.
- **Cache**: `ListSkills` walk now propagates non-permission filesystem errors instead of silently ignoring all errors.
- **Repo**: Fixed overly permissive repository name matching that could match substrings (e.g., "repo" matching "another-repo").
- **Completion**: Shell completion generation errors are now reported instead of silently ignored.

### Changed
- **Offline Mode**: Consolidated duplicate `OfflineMode` globals (`config.OfflineMode` and `github.OfflineMode`) into single source of truth (`config.OfflineMode`).
- **Config**: Extracted shared `loadConfigFromPath`/`mergeDefaults` helpers to eliminate code duplication between `LoadConfig` and `LoadGlobalConfig`.
- **Build**: Added `.PHONY` declarations to Makefile.
- **Server**: Removed dead no-op code in skill search handler.

## [1.7.2] - 2026-02-19

### Fixed
- **CI**: Fixed race condition in GitHub Actions release workflow that caused Goreleaser to fail when desktop builds finished first.
- **Linting**: Fixed string formatting in `internal/skill/report.go` to use `fmt.Fprintf` instead of `WriteString(fmt.Sprintf(...))`.
- **Stability**: Resolved potential nil pointer dereference (`SA5011`) issues in `cmd/install.go` and `cmd/repo.go`.

## [1.7.1] - 2026-02-19

### Changed
- **Release**: Patch release with internal improvements.

## [1.7.0] - 2026-02-12

### Changed
- **Release**: Consolidate v1.6.x feature additions into minor release.

## [1.6.5] - 2026-02-12

### Added
- **Repository Filter**: Added `--repo` flag to `ask skill install` to filter skills by repository or install all skills from a specific repository.

## [1.6.4] - 2026-02-12

### Added
- **Skill Restoration**: `ask skill install` (without arguments) now restores skills from `ask.lock` or `ask.yaml` in the current directory.

## [1.6.3] - 2026-02-12

### Changed
- **Release**: Patch release with version bump and release preparation.

## [1.6.2] - 2026-02-09

### Added
- **Alias**: Added `ask update` as a top-level alias for `ask skill update` for convenience.

## [1.6.1] - 2026-02-09

### Changed
- **Performance**: Optimized `ask repo sync` with parallel processing (5x concurrency) and unified progress bar.
- **Git**: Improved git operation handling to prevent output interleaving during concurrent syncs.

## [1.6.0] - 2026-02-07

### Added
- **Internationalization**: Added complete Chinese documentation for all `docs/` files (e.g., `README_zh.md`, `commands_zh.md`).
- **Prompt Integration**: New `ask skill prompt` command to generate XML skill listings for Agentic AI prompts (following [Agent Skills Spec](https://agentskills.io/specification)).
- **Validation**: Enhanced `SKILL.md` validation logic to rigidly enforce spec compliance (names, descriptions).

### Fixed
- **Cleanup**: Removed unused `formatPathForPrompt` function in `cmd/prompt.go`.

## [1.5.1] - 2026-02-04

### Changed
- **Server**: Refactored `server.go` into `handlers_skill.go`, `handlers_repo.go`, and `handlers_system.go` for better maintainability.
- **Service**: Added comprehensive unit tests for `internal/service` package (achieving 100% coverage).
- **Server**: Fixed duplicate comments and added error logging for `git sync` failures in `handleRepoSync`.

## [1.5.0] - 2026-02-04

### Changed
- **CI**: Unified Go version to 1.24 across all CI workflows (lint.yml was using 1.21).
- **Documentation**: Fixed CHANGELOG.md version links (added missing 1.4.2, 1.4.3 entries).
- **Maintenance**: Cleaned up `.gitignore` by removing redundant log file entries.

## [1.4.3] - 2026-02-03

### Added
- **Documentation**: Added Antigravity Awesome Skills to optional repositories.
- **Offline**: Improved offline mode to properly short-circuit network requests in SkillHub client.

### Changed
- **Serve**: Changed default port to `8125` to match documentation and prevent automation regressions.
- **Security**: Reduced false positives for HTTP links in security scanner (only warns for non-localhost/non-private IPs).

## [1.4.2] - 2026-02-03

### Fixed
- **Documentation**: Fixed incorrect release badge and download links in README.md and README_zh.md (was pointing to wrong repository).
- **CI**: Fixed Codecov upload condition in test.yml (go-version 1.22 → 1.24).
- **Documentation**: Fixed malformed YAML frontmatter example in SPEC.md.

## [1.4.1] - 2026-02-01

### Fixed
- **Config**: Corrected repository URLs and CI configuration.

## [1.4.0] - 2026-01-31

### Fixed
- **Monorepo Support**: Fixed `ask repo sync` failing to retrieve star counts for repositories configured with subpaths (e.g., `owner/repo/path/to/skills`).
- **URL Parsing**: Improved robustness of GitHub URL parsing for various formats.
- **Build**: Fixed compilation error in `server` package initialization.
- **Web UI**: Updated server initialization to include version information.

## [1.3.3] - 2026-01-30

### Fixed
- **Desktop**: Fixed "Failed to update project root" error in settings by properly handling configuration context switching.
- **Linting**: Fixed various lint errors in filesystem, installer, and completion packages.

## [1.3.2] - 2026-01-30

### Fixed
- **Build**: Updated GoReleaser config to use `brews` instead of `homebrew_casks` for correct Formula generation.
- **CI**: Updated `release.yml` to use `libwebkit2gtk-4.1-dev` for compatibility with Ubuntu 24.04 (Noble).

## [1.3.1] - 2026-01-30

### Fixed
- **Web UI**: Fixed missing icons in Repositories view by correctly prioritizing GitHub URLs for avatar generation.
- **Server**: Fixed unused variable lint error in server code.

## [1.3.0] - 2026-01-30

### Added
- **Desktop**: Added desktop app support with Wails framework.

### Fixed
- **Build**: Fixed GoReleaser v2 deprecation and independent desktop builds.
- **Windows**: Fixed cross-platform build for Windows (Setpgid/SIGTERM).

## [1.2.0] - 2026-01-29

### Added
- **Web UI**: Added web-based UI for skill management via `ask serve`.
- **Install**: Added installation support for Windows and Linux.
- **Agents**: Added CodeBuddy agent support.

## [1.1.3] - 2026-01-26

### Added
- **Security Report Improvements**:
    - **Report Completeness**: HTML reports now list all scanned modules, including "safe" ones, providing a complete audit trail.
    - **Enhanced Visualization**: Clean modules are styled with a light green background and "Safe" badge for quick identification.
    - **Optimized Sorting**: Modules are now sorted by risk level (Critical -> Warning -> Info -> Clean) to prioritize attention on issues.

## [1.1.2] - 2026-01-26

### Added
- **Repo Sync**: Added `--sync` flag to `ask repo add` command for immediate synchronization.
- **Fuzzy Matching**: Enhanced `ask repo list` to support fuzzy matching by URL or `owner/repo` pattern.

### Changed
- **Case Sensitivity**: Fixed an issue where `cmd_test.go` failed due to case sensitivity of agent names.
- **Documentation**: Updated `README.md` and `README_zh.md` to clarify repository sync behavior.

## [1.1.1] - 2026-01-26

### Fixed
- **Repo**: Improved repo list fuzzy matching.
- **Docs**: Updated documentation with correct Go version and synced with v1.1.0 changes.

## [1.1.0] - 2026-01-26

### Added
- **Enhanced Security Reports**: 
    - Generated HTML reports now include collapsible Module/Severity sections for better navigation.
    - Added comprehensive Overview dashboard with severity distribution charts.
    - Improved report header with "by ASK" attribution and precise timestamp.
    - Path display optimization: Reports now intelligently show paths (e.g., `~/Projects/...`) instead of just `.` or full absolute paths.
- **Batch Scanning**: Support for batch security scanning of multiple repositories.
- **Documentation**: Added "Security Reports" section to `README.md` and `README_zh.md` with links to live samples.

### Changed
- Refined HTML report CSS for better readability and interactivity.
- Defaulted findings groups to collapsed state for cleaner initial view.

## [1.0.0] - 2026-01-25

### Added
- **Security Checks**: New `ask check` command to scan skills for secrets, dangerous commands, and suspicious files.
- **Values Reports**: Generate detailed security reports in Markdown, HTML, or JSON with `ask check -o <file>`.
- **Entropy Analysis**: Smart secret detection using Shannon entropy to reduce false positives.

## [1.0.0-rc2] - 2026-01-25

### Added
- **Docker-Style Aliases**: New top-level commands `ask install`, `ask search`, `ask list` for faster access.
- **Install Aliases**: `ask add` (and `ask i`) supported as aliases for `install`.
- **Smart Sync**: `ask search` now automatically initializes the local cache if empty (Lazy Init) and updates it in the background if stale (> 3 days).
- **Auto-Caching on Install**: Installation of a skill from an uncached repository now triggers a background sync.
- **Local Install Optimization**: Installing a skill that exists in the local cache now performs a fast file copy instead of a git clone.
- **Testing**: Added comprehensive unit tests for CLI commands.

### Fixed
- **Panic on Single-Word Install**: Fixed critical panic when using `ask install <name>` with a single word argument.
- **Uninstall Alias**: Added missing top-level `ask uninstall` alias (previously only `ask skill uninstall` worked).
- **Documentation**: Removed invalid `skillhub/skills` repository example and clarified `mcp-builder` installation.
- **Input Validation**: Added input length limits and stricter validation to prevent empty skill name installations from malformed inputs.
- **Robustness**: Improved re-installation check safety.
- **Robust Installation**: Fixed issue where `ask skill install Source/Skill` would fail if the local cache was empty/missing.
- **Index Reliability**: Fixed a bug where repository URLs were not being persisted to `index.json`.

### Changed
- **Repository Naming**: Local cache directories now use the user-configured repository name (e.g. `anthropics`).
- **Improved UX**: Reduced verbosity of installation commands.
- **Search UI**: Removed `local:` prefix from search results.
- **Documentation**: Updated English and Chinese READMEs with new alias usage.

## [1.0.0-rc1] - 2026-01-24

### Added
- **Initial Release Candidate**: First RC for v1.0.0 with core functionality stabilized.
- **Skill Management**: `ask skill install`, `ask skill search`, `ask skill list` commands.
- **Repository System**: Multi-source repository configuration and caching.

## [0.9.0] - 2026-01-21

### Changed
- **Config Tests**: Fixed and updated `internal/config` tests to align with default repository configuration.
- **Code Quality**: Resolved linter warnings including `io/ioutil` deprecation and unhandled errors.
- **Stability**: general improvements and test coverage updates.

## [0.8.0] - 2026-01-19

### Added
- **SkillHub Integration**: Added support for searching and installing skills from [SkillHub.club](https://www.skillhub.club).
- **Slug Resolution**: intelligent resolution of SkillHub slugs to GitHub install URLs.

## [0.7.6] - 2026-01-19

### Changed
- **Skill Discovery**: Now uses `git clone` for skill discovery from configured repositories, eliminating the need for GitHub tokens for public repositories and avoiding API rate limits.

## [0.7.5] - 2026-01-19

### Added
- **Vercel Skills**: Added `vercel-labs/agent-skills` to default repositories
- **Documentation**: Added CLI demo screenshot, sorted skill sources alphabetically

## [0.7.4] - 2026-01-19

### Added
- **Composio Skills**: Added `ComposioHQ/awesome-claude-skills` to default repositories as `composio`
- **Documentation**: Updated skill sources documentation to include Composio

## [0.7.3] - 2026-01-19

### Added
- **Bulk Skill Installation**: `ask skill install <repo>` now installs all skills from the repository (e.g. `ask skill install superpowers`)
- **MATLAB Skills**: Added official `matlab` repository to default skill sources
- **Documentation**: Added detailed install path documentation for different agents (`.claude`, `.cursor`, etc.) in `README.md` and `README_zh.md`

### Fixed
- Fixed specific config URL handling in `ask skill install` matching logic

## [0.7.2] - 2026-01-17

### Added
- `ask skills` command alias
- GitHub token authentication support (`GITHUB_TOKEN` or `GH_TOKEN`) to avoid rate limit errors

## [0.7.1] - 2026-01-17

### Fixed
- Fixed GoReleaser config to place Homebrew formula in `Formula/` directory

## [0.7.0] - 2026-01-17

### Fixed
- Homebrew formula now uses pre-compiled binaries instead of source compilation
- Users no longer need Go installed to install via Homebrew

## [0.6.1] - 2026-01-16

### Fixed
- Fixed GitHub Actions release workflow to use `GH_PAT` for Homebrew tap updates

## [0.6.0] - 2026-01-16

### Added
- `ask repo list [name]` command to inspect repository skills
- Shell completion support for bash, zsh, fish, and powershell
- Comprehensive test coverage for `internal/git`, `internal/skill`, `internal/deps`, and `internal/ui` packages
- CI/CD quality gates: linting, test coverage reporting, and security scanning
- Documentation: troubleshooting guide and architecture diagrams
- Git author extraction from git config for `ask skill create` command

### Changed
- `ask repo list` now supports optional arguments to list skills in a specific repository
- Enhanced command help text with practical examples
- Improved error messages with actionable suggestions

## [0.5.0] - 2026-01-16

### Added
- **Multi-Tool Support** (`--agent` / `-a` flag)
  - Automatically detects and installs skills for: Claude Code, Cursor, OpenAI Codex, OpenCode
  - Supports installing to multiple agents simultaneously
- **Global Installation Support** (`--global` / `-g` flag)
  - Install skills globally to `~/.ask/skills/` for sharing across all projects
  - Global configuration stored in `~/.ask/config.yaml`
  - Global lock file at `~/.ask/ask.lock`
  - `ask skill list --all` to show both project and global skills
- All skill commands now support `--global` flag: `install`, `uninstall`, `list`, `update`, `outdated`, `info`

### Changed
- Skill commands now display installation scope (project/global) in output messages

## [0.4.0] - 2026-01-15

### Added
- Offline Mode (`--offline` flag)
- `ask benchmark` command
- Search caching for offline support

### Changed
- `install` and `outdated` commands respect offline mode

## [0.2.0] - 2026-01-15

### Added
- Skill commands moved to `ask skill` subcommand for better organization
- New default repositories: OpenAI, GitHub Copilot skills
- `ask skill outdated` command to check for available updates
- `ask skill update` command to update skills to latest versions
- `ask skill create` command to generate skill templates
- Version locking support with `ask.lock` file
- Git sparse checkout optimization for faster skill installation
- Search result caching for improved performance
- Progress bars for long-running operations
- Configurable skills directory (default: `.agent/skills/`)

### Changed
- CLI restructure: all skill operations now under `ask skill` parent command
- Skills installation path changed from `./skills/` to `.agent/skills/`
- Default repositories expanded to include community, Anthropic, MCP, Scientific, Superpowers, and OpenAI sources

### Fixed
- Uninstall command now properly removes skills from both filesystem and config
- Same-name skills from different sources are now properly distinguished
- Repository management commands (`ask repo add/list/remove`) now work correctly

## [0.1.0] - 2026-01-15

### Added
- Initial release of ASK (Agent Skills Kit)
- Basic skill management: search, install, uninstall, list, info
- Multi-source skill discovery (GitHub topics and directories)
- Project initialization with `ask init`
- Repository management with `ask repo` commands
- Configuration file support (`ask.yaml`)
- Default repositories: Community, Anthropic, MCP-Servers, Scientific, Superpowers

[Unreleased]: https://github.com/yeasy/ask/compare/v1.9.6...HEAD
[1.9.6]: https://github.com/yeasy/ask/compare/v1.9.5...v1.9.6
[1.9.5]: https://github.com/yeasy/ask/compare/v1.9.4...v1.9.5
[1.9.4]: https://github.com/yeasy/ask/compare/v1.9.3...v1.9.4
[1.9.3]: https://github.com/yeasy/ask/compare/v1.9.2...v1.9.3
[1.9.2]: https://github.com/yeasy/ask/compare/v1.9.1...v1.9.2
[1.9.1]: https://github.com/yeasy/ask/compare/v1.9.0...v1.9.1
[1.9.0]: https://github.com/yeasy/ask/compare/v1.8.1...v1.9.0
[1.8.1]: https://github.com/yeasy/ask/compare/v1.8.0...v1.8.1
[1.8.0]: https://github.com/yeasy/ask/compare/v1.7.9...v1.8.0
[1.7.9]: https://github.com/yeasy/ask/compare/v1.7.8...v1.7.9
[1.7.8]: https://github.com/yeasy/ask/compare/v1.7.7...v1.7.8
[1.7.7]: https://github.com/yeasy/ask/compare/v1.7.6...v1.7.7
[1.7.6]: https://github.com/yeasy/ask/compare/v1.7.5...v1.7.6
[1.7.5]: https://github.com/yeasy/ask/compare/v1.7.4...v1.7.5
[1.7.4]: https://github.com/yeasy/ask/compare/v1.7.3...v1.7.4
[1.7.3]: https://github.com/yeasy/ask/compare/v1.7.2...v1.7.3
[1.7.2]: https://github.com/yeasy/ask/compare/v1.7.1...v1.7.2
[1.7.1]: https://github.com/yeasy/ask/compare/v1.7.0...v1.7.1
[1.7.0]: https://github.com/yeasy/ask/compare/v1.6.5...v1.7.0
[1.6.5]: https://github.com/yeasy/ask/compare/v1.6.4...v1.6.5
[1.6.4]: https://github.com/yeasy/ask/compare/v1.6.3...v1.6.4
[1.6.3]: https://github.com/yeasy/ask/compare/v1.6.2...v1.6.3
[1.6.2]: https://github.com/yeasy/ask/compare/v1.6.1...v1.6.2
[1.6.1]: https://github.com/yeasy/ask/compare/v1.6.0...v1.6.1
[1.6.0]: https://github.com/yeasy/ask/compare/v1.5.1...v1.6.0
[1.5.1]: https://github.com/yeasy/ask/compare/v1.5.0...v1.5.1
[1.5.0]: https://github.com/yeasy/ask/compare/v1.4.3...v1.5.0
[1.4.3]: https://github.com/yeasy/ask/compare/v1.4.2...v1.4.3
[1.4.2]: https://github.com/yeasy/ask/compare/v1.4.1...v1.4.2
[1.4.1]: https://github.com/yeasy/ask/compare/v1.4.0...v1.4.1
[1.4.0]: https://github.com/yeasy/ask/compare/v1.3.3...v1.4.0
[1.3.3]: https://github.com/yeasy/ask/compare/v1.3.2...v1.3.3
[1.3.2]: https://github.com/yeasy/ask/compare/v1.3.1...v1.3.2
[1.3.1]: https://github.com/yeasy/ask/compare/v1.3.0...v1.3.1
[1.3.0]: https://github.com/yeasy/ask/compare/v1.2.0...v1.3.0
[1.2.0]: https://github.com/yeasy/ask/compare/v1.1.3...v1.2.0
[1.1.3]: https://github.com/yeasy/ask/compare/v1.1.2...v1.1.3
[1.1.2]: https://github.com/yeasy/ask/compare/v1.1.1...v1.1.2
[1.1.1]: https://github.com/yeasy/ask/compare/v1.1.0...v1.1.1
[1.1.0]: https://github.com/yeasy/ask/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/yeasy/ask/compare/v1.0.0-rc2...v1.0.0
[1.0.0-rc2]: https://github.com/yeasy/ask/compare/v1.0.0-rc1...v1.0.0-rc2
[1.0.0-rc1]: https://github.com/yeasy/ask/compare/v0.9.0...v1.0.0-rc1
[0.9.0]: https://github.com/yeasy/ask/compare/v0.8.0...v0.9.0
[0.8.0]: https://github.com/yeasy/ask/compare/v0.7.6...v0.8.0
[0.7.6]: https://github.com/yeasy/ask/compare/v0.7.5...v0.7.6
[0.7.5]: https://github.com/yeasy/ask/compare/v0.7.4...v0.7.5
[0.7.4]: https://github.com/yeasy/ask/compare/v0.7.3...v0.7.4
[0.7.3]: https://github.com/yeasy/ask/compare/v0.7.2...v0.7.3
[0.7.2]: https://github.com/yeasy/ask/compare/v0.7.1...v0.7.2
[0.7.1]: https://github.com/yeasy/ask/compare/v0.7.0...v0.7.1
[0.7.0]: https://github.com/yeasy/ask/compare/v0.6.1...v0.7.0
[0.6.1]: https://github.com/yeasy/ask/compare/v0.6.0...v0.6.1
[0.6.0]: https://github.com/yeasy/ask/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/yeasy/ask/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/yeasy/ask/compare/v0.2.0...v0.4.0
[0.2.0]: https://github.com/yeasy/ask/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/yeasy/ask/releases/tag/v0.1.0
