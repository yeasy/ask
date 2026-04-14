# Contributing to ASK (Agent Skills Kit)

Thank you for your interest in contributing to ASK! This document provides guidelines and instructions for contributing.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Making Changes](#making-changes)
- [Testing](#testing)
- [Submitting Changes](#submitting-changes)
- [Style Guide](#style-guide)

## Code of Conduct

By participating in this project, you agree to maintain a respectful and inclusive environment for all contributors.

## Getting Started

### Prerequisites

- Go 1.25 or higher
- Git
- A GitHub account

### Development Setup

1. **Fork the repository**
   ```bash
   # Fork on GitHub, then clone your fork
   git clone https://github.com/YOUR_USERNAME/ask.git
   cd ask
   ```

2. **Install dependencies**
   ```bash
   go mod download
   ```

3. **Build the project**
   ```bash
   make build
   ```

4. **Run tests**
   ```bash
   make test
   ```

5. **Verify everything works**
   ```bash
   ./ask --help
   ```

## Making Changes

### Branch Naming

Use descriptive branch names:
- `feature/add-xyz` - for new features
- `fix/issue-123` - for bug fixes
- `docs/update-readme` - for documentation
- `refactor/cleanup-xyz` - for refactoring

### Commit Messages

Follow the [Conventional Commits](https://www.conventionalcommits.org/) specification:

```
<type>(<scope>): <description>

[optional body]

[optional footer]
```

**Types:**
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `test`: Adding or updating tests
- `refactor`: Code refactoring
- `chore`: Maintenance tasks
- `ci`: CI/CD changes

**Examples:**
```bash
feat(search): add regex support for skill search
fix(install): handle network timeout gracefully
docs: update installation guide for Windows
test: add integration tests for repo commands
```

## Testing

### Running Tests

```bash
# Run all tests
make test

# Run tests with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run specific package tests
go test ./internal/config -v

# Run individual test
go test ./internal/config -run TestDefaultConfig -v
```

### Writing Tests

- Place test files next to the code they test (`filename_test.go`)
- Use table-driven tests where appropriate
- Aim for at least 60% code coverage
- Test both success and error cases
- Mock external dependencies (git, GitHub API)

**Example:**
```go
func TestSkillInstall(t *testing.T) {
    tests := []struct {
        name    string
        skill   string
        wantErr bool
    }{
        {"valid skill", "browser-use", false},
        {"invalid skill", "", true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := installSkill(tt.skill)
            if (err != nil) != tt.wantErr {
                t.Errorf("installSkill() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### Test Requirements

All PRs must:
- Include tests for new features
- Maintain or improve code coverage
- Pass all existing tests
- Pass `go vet` and `go fmt` checks

## Submitting Changes

### Pull Request Process

1. **Update your fork**
   ```bash
   git checkout main
   git pull upstream main
   git push origin main
   ```

2. **Create a feature branch**
   ```bash
   git checkout -b feature/your-feature-name
   ```

3. **Make your changes**
   - Write clean, readable code
   - Add tests
   - Update documentation

4. **Commit your changes**
   ```bash
   git add .
   git commit -m "feat: your feature description"
   ```

5. **Push to your fork**
   ```bash
   git push origin feature/your-feature-name
   ```

6. **Create a Pull Request**
   - Go to the [ASK repository](https://github.com/yeasy/ask)
   - Click "New Pull Request"
   - Select your branch
   - Fill in the PR template
   - Link related issues

### PR Checklist

Before submitting, ensure:
- [ ] Code builds successfully (`make build`)
- [ ] All tests pass (`make test`)
- [ ] Code is formatted (`make fmt`)
- [ ] No linter warnings (`make lint`)
- [ ] Documentation is updated (if needed)
- [ ] CHANGELOG.md is updated (if applicable)
- [ ] Commit messages follow convention
- [ ] Branch is up-to-date with main

## Style Guide

### Go Code Style

Follow the official [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments).

**Key points:**
- Use `gofmt` for formatting
- Use meaningful variable names
- Add comments for exported functions
- Keep functions small and focused
- Handle errors explicitly
- Use early returns to reduce nesting

**Example:**
```go
// SearchSkills searches for skills matching the given keyword across all sources.
// Returns a slice of skills or an error if the search fails.
func SearchSkills(keyword string) ([]Skill, error) {
    if keyword == "" {
        return nil, fmt.Errorf("keyword cannot be empty")
    }
    
    // Search logic here...
    
    return skills, nil
}
```

### File Organization

```
ask/
├── cmd/                 # Command implementations
│   ├── root.go         # Root command
│   ├── skill.go        # Skill subcommand
│   └── *.go            # Other commands
├── internal/           # Private packages
│   ├── app/            # Application orchestration
│   ├── cache/          # Search result caching
│   ├── config/         # Configuration handling
│   ├── deps/           # Dependency resolution
│   ├── filesystem/     # File operations
│   ├── git/            # Git operations
│   ├── github/         # GitHub API client
│   ├── installer/      # Skill installation logic
│   ├── repository/     # Repository management
│   ├── server/         # HTTP server (web UI)
│   ├── service/        # Process management
│   ├── skill/          # SKILL.md parsing & security
│   ├── skillhub/       # SkillHub registry client
│   └── ui/             # Terminal UI helpers
├── docs/               # Documentation
└── main.go             # Entry point
```

### Documentation

- Update README.md for user-facing changes
- Update docs/ for detailed guides
- Add godoc comments for exported functions
- Include examples in documentation

## Getting Help

- **Questions**: Open a [Discussion](https://github.com/yeasy/ask/discussions)
- **Bugs**: Open an [Issue](https://github.com/yeasy/ask/issues)
- **Security**: See [SECURITY.md](SECURITY.md)

## Recognition

Contributors will be acknowledged in:
- Release notes
- Contributors list
- Special mentions for significant contributions

Thank you for contributing to ASK! 🚀
