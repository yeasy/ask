# Troubleshooting Guide

This guide helps you resolve common issues when using ASK (Agent Skills Kit).

## Installation Issues

### Homebrew Installation Fails

**Problem**: `brew install ask` fails or the command is not found.

**Solutions**:
```bash
# 1. Make sure you've added the tap
brew tap yeasy/tap

# 2. Update Homebrew
brew update

# 3. Try installing again
brew install ask

# 4. If still failing, install from source
git clone https://github.com/yeasy/ask.git
cd ask
make build
sudo mv ask /usr/local/bin/
```

### Build from Source Fails

**Problem**: `make build` fails with compilation errors.

**Solutions**:
```bash
# Check Go version (requires 1.25+)
go version

# Update Go if needed (macOS)
brew upgrade go

# Clean and rebuild
make clean
go mod download
make build
```

---

## Skill Installation Issues

### Git Clone Fails

**Problem**: Skill installation fails with "git clone" errors.

**Possible Causes & Solutions**:

1. **GitHub Rate Limiting**
   ```bash
   # Check your GitHub API rate limit
   curl -H "Authorization: token YOUR_GITHUB_TOKEN" \
     https://api.github.com/rate_limit
   
   # Set GitHub token to increase rate limit
   export GITHUB_TOKEN=your_personal_access_token
   ```

2. **Network Connectivity**
   ```bash
   # Test connection to GitHub
   ping github.com
   
   # Try using HTTPS instead of SSH
   git config --global url."https://github.com/".insteadOf git@github.com:
   ```

3. **Repository Doesn't Exist**
   ```bash
   # Verify the repository exists
   ask skill search <skill-name>
   
   # Check the exact URL
   ask skill info <skill-name>
   ```

### Sparse Checkout Fails

**Problem**: Installation fails with "sparse-checkout" errors.

**Solution**:
```bash
# Check Git version (sparse checkout requires Git 2.25+)
git --version

# Update Git if needed (macOS)
brew upgrade git

# The tool will automatically fall back to full clone if sparse fails
```

### "Skill Already Installed" Error

**Problem**: Cannot reinstall a skill.

**Solution**:
```bash
# Uninstall the skill first
ask skill uninstall <skill-name>

# Then reinstall
ask skill install <skill-name>

# Or manually remove and reinstall
rm -rf .agent/skills/<skill-name>
ask skill install <skill-name>
```

---

## Search Issues

### No Results Found

**Problem**: `ask skill search` returns no results.

**Solutions**:
```bash
# 1. Check your internet connection
ping api.github.com

# 2. List configured repositories
ask repo list

# 3. Try searching without keywords
ask skill search

# 4. Clear search cache
rm -rf ~/.cache/ask

# 5. Check if GitHub API is accessible
curl https://api.github.com/zen
```

### Slow Search Performance

**Problem**: Search takes a long time to complete.

**Solutions**:
- **Use Caching**: Results are cached for 1 hour by default
- **Be Specific**: Use more specific keywords to reduce results
- **Check Network**: Slow network can affect GitHub API calls

```bash
# Test network speed to GitHub
time curl -s https://api.github.com > /dev/null
```

---

## Configuration Issues

### ask.yaml Not Found

**Problem**: Commands fail with "ask.yaml not found".

**Solution**:
```bash
# Initialize your project
ask init

# This creates ask.yaml in the current directory
# Make sure you're in the right project directory
pwd
ls -la ask.yaml
```

### Invalid Repository URL

**Problem**: "Invalid repository format" error when adding a repo.

**Solution**:
```bash
# Use the correct format: owner/repo or owner/repo/path
ask repo add anthropics/skills

# For subdirectories
ask repo add anthropics/skills/skills

# Not full URLs
# ❌ ask repo add https://github.com/anthropics/skills
# ✅ ask repo add anthropics/skills
```

---

## Permission Issues

### Cannot Create .agent Directory

**Problem**: Permission denied when installing skills.

**Solution**:
```bash
# Check directory permissions
ls -la .

# Create directory with proper permissions
mkdir -p .agent/skills
chmod 755 .agent

# Avoid using sudo with ask commands
```

### Cannot Execute Script Files

**Problem**: Script files in skills cannot be executed.

**Solution**:
```bash
# Make scripts executable
chmod +x .agent/skills/*/scripts/*.sh

# For a specific skill
chmod -R +x .agent/skills/browser-use/scripts/
```

---

## Version & Update Issues

### Cannot Update Skill

**Problem**: `ask skill update` fails.

**Solutions**:
```bash
# 1. Check if skill is a git repository
cd .agent/skills/<skill-name>
git status

# 2. If it's not a git repo, reinstall it
ask skill uninstall <skill-name>
ask skill install <skill-name>

# 3. If git is out of sync, reset it
cd .agent/skills/<skill-name>
git fetch origin
git reset --hard origin/main
```

### Version Lock Not Working

**Problem**: Specific version not installed despite `@version` syntax.

**Solution**:
```bash
# Ensure the version/tag exists
# Check GitHub releases page

# Use exact tag name
ask skill install anthropics/skills@v1.0.0

# If tag doesn't exist, you'll get the default branch
```

---

## Lock File Issues

### ask.lock Out of Sync

**Problem**: Lock file doesn't match installed skills.

**Solution**:
```bash
# Regenerate lock file by reinstalling skills
ask skill list  # Note your installed skills

# Uninstall and reinstall each
ask skill uninstall <skill-name>
ask skill install <skill-name>@<version>
```

---

## GitHub API Issues

### Rate Limit Exceeded

**Problem**: "API rate limit exceeded" error.

**Solution**:
```bash
# Create a GitHub personal access token
# https://github.com/settings/tokens

# Set it as an environment variable
export GITHUB_TOKEN=ghp_your_token_here

# Add to your shell profile for persistence
echo 'export GITHUB_TOKEN=ghp_your_token_here' >> ~/.zshrc
source ~/.zshrc

# Verify it's set
echo $GITHUB_TOKEN
```

### Authentication Failed

**Problem**: "Authentication required" or 403 errors.

**Solution**:
```bash
# For private repositories, set GitHub token
export GITHUB_TOKEN=your_token_with_repo_access

# Check Git credentials
git config --global user.name
git config --global user.email

# Update Git credentials if needed
git config --global credential.helper cache
```

---

## General Debugging

### Enable Verbose Logging
Most operational messages (scanning, updating, searching) are hidden by default to keep the output clean.
To see detailed logs for debugging purposes, set the log level to `DEBUG`:

```bash
# Run commands with verbose output
ask --log-level debug skill install browser-use
```

### Check System Requirements

```bash
# Verify all requirements
go version      # Should be 1.25+
git --version   # Should be 2.25+
which ask       # Should show installation path

# Check environment
env | grep GITHUB
```

### Common Environment Variables

```bash
# Set GitHub token for API access (GH_TOKEN also supported)
export GITHUB_TOKEN=your_token
```

---

## Getting Help

If you're still experiencing issues:

1. **Check existing issues**: https://github.com/yeasy/ask/issues
2. **Search discussions**: https://github.com/yeasy/ask/discussions
3. **Open a new issue**: Provide:
   - ASK version (`ask version`)
   - Operating system
   - Go version (`go version`)
   - Git version (`git --version`)
   - Complete error message
   - Steps to reproduce

4. **Community support**:
   - GitHub Discussions
   - Stack Overflow (tag: `ask-cli`)

---

## Quick Reference

### Reset Everything

```bash
# Complete reset (careful!)
rm -rf .agent/skills
rm ask.yaml
rm ask.lock
ask init
```

### Verify Installation

```bash
# Check ASK is working
ask --help
ask skill search browser
ask repo list
```

### Common Workflow

```bash
# 1. Initialize project
ask init

# 2. Search for skills
ask skill search <keyword>

# 3. Install skills
ask skill install <skill-name>

# 4. Verify installation
ask skill list

# 5. Keep skills updated
ask skill outdated
ask skill update
```
