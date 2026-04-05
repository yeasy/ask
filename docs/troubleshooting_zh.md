# 故障排除指南

本指南旨在帮助您解决在使用 ASK (Agent Skills Kit) 时遇到的常见问题。

## 安装问题

### Homebrew 安装失败

**问题**：`brew install ask` 失败或找不到命令。

**解决方案**：
```bash
# 1. 确保已添加 tap
brew tap yeasy/tap

# 2. 更新 Homebrew
brew update

# 3. 尝试重新安装
brew install ask

# 4. 如果仍然失败，请从源码安装
git clone https://github.com/yeasy/ask.git
cd ask
make build
sudo mv ask /usr/local/bin/
```

### 源码构建失败

**问题**：`make build` 失败并出现编译错误。

**解决方案**：
```bash
# 检查 Go 版本 (需要 1.25+)
go version

# 如果需要，更新 Go (macOS)
brew upgrade go

# 清理并重新构建
make clean
go mod download
make build
```

---

## 技能安装问题

### Git Clone 失败

**问题**：技能安装失败并出现 "git clone" 错误。

**可能的原因及解决方案**：

1. **GitHub 速率限制**
   ```bash
   # 检查您的 GitHub API 速率限制
   curl -H "Authorization: token YOUR_GITHUB_TOKEN" \
     https://api.github.com/rate_limit
   
   # 设置 GitHub 令牌以增加速率限制
   export GITHUB_TOKEN=your_personal_access_token
   ```

2. **网络连接**
   ```bash
   # 测试连接到 GitHub
   ping github.com
   
   # 尝试使用 HTTPS 代替 SSH
   git config --global url."https://github.com/".insteadOf git@github.com:
   ```

3. **仓库不存在**
   ```bash
   # 验证仓库是否存在
   ask skill search <skill-name>
   
   # 检查确切的 URL
   ask skill info <skill-name>
   ```

### Sparse Checkout 失败

**问题**：安装失败并出现 "sparse-checkout" 错误。

**解决方案**：
```bash
# 检查 Git 版本 (sparse checkout 需要 Git 2.25+)
git --version

# 如果需要，更新 Git (macOS)
brew upgrade git

# 如果 sparse 失败，工具会自动回退到完全克隆
```

### "Skill Already Installed" 错误

**问题**：无法重新安装技能。

**解决方案**：
```bash
# 先卸载技能
ask skill uninstall <skill-name>

# 然后重新安装
ask skill install <skill-name>

# 或者手动删除并重新安装
rm -rf .agent/skills/<skill-name>
ask skill install <skill-name>
```

---

## 搜索问题

### 未找到结果

**问题**：`ask skill search` 未返回任何结果。

**解决方案**：
```bash
# 1. 检查您的互联网连接
ping api.github.com

# 2. 列出已配置的仓库
ask repo list

# 3. 尝试不带关键字进行搜索
ask skill search

# 4. 清除搜索缓存
rm -rf ~/.cache/ask

# 5. 检查 GitHub API 是否可访问
curl https://api.github.com/zen
```

### 搜索速度慢

**问题**：搜索需要很长时间才能完成。

**解决方案**：
- **使用缓存**：结果默认缓存 1 小时
- **更具体**：使用更具体的关键字以减少结果
- **检查网络**：网络缓慢会影响 GitHub API 调用

```bash
# 测试到 GitHub 的网络速度
time curl -s https://api.github.com > /dev/null
```

---

## 配置问题

### ask.yaml 未找到

**问题**：命令失败并提示 "ask.yaml not found"。

**解决方案**：
```bash
# 初始化您的项目
ask init

# 这将在当前目录中创建 ask.yaml
# 确保您在正确的项目目录中
pwd
ls -la ask.yaml
```

### 无效的仓库 URL

**问题**：添加仓库时出现 "Invalid repository format" 错误。

**解决方案**：
```bash
# 使用正确的格式：owner/repo 或 owner/repo/path
ask repo add anthropics/skills

# 对于子目录
ask repo add anthropics/skills/skills

# 不是完整的 URL
# ❌ ask repo add https://github.com/anthropics/skills
# ✅ ask repo add anthropics/skills
```

---

## 权限问题

### 无法创建 .agent 目录

**问题**：安装技能时权限被拒绝。

**解决方案**：
```bash
# 检查目录权限
ls -la .

# 创建具有适当权限的目录
mkdir -p .agent/skills
chmod 755 .agent

# 避免使用 sudo 运行 ask 命令
```

### 无法执行脚本文件

**问题**：技能中的脚本文件无法执行。

**解决方案**：
```bash
# 使脚本可执行
chmod +x .agent/skills/*/scripts/*.sh

# 对于特定技能
chmod -R +x .agent/skills/browser-use/scripts/
```

---

## 版本和更新问题

### 无法更新技能

**问题**：`ask skill update` 失败。

**解决方案**：
```bash
# 1. 检查技能是否为 git 仓库
cd .agent/skills/<skill-name>
git status

# 2. 如果它不是 git 仓库，请重新安装它
ask skill uninstall <skill-name>
ask skill install <skill-name>

# 3. 如果 git 不同步，请重置它
cd .agent/skills/<skill-name>
git fetch origin
git reset --hard origin/main
```

### 版本锁定不起作用

**问题**：尽管使用了 `@version` 语法，但未安装特定版本。

**解决方案**：
```bash
# 确保版本/标签存在
# 检查 GitHub releases 页面

# 使用确切的标签名称
ask skill install anthropics/skills@v1.0.0

# 如果标签不存在，您将获得默认分支
```

---

## 锁定文件问题

### ask.lock 不同步

**问题**：锁定文件与已安装的技能不匹配。

**解决方案**：
```bash
# 通过重新安装技能重新生成锁定文件
ask skill list  # 记录您已安装的技能

# 卸载并重新安装每一个
ask skill uninstall <skill-name>
ask skill install <skill-name>@<version>
```

---

## GitHub API 问题

### 速率限制超出

**问题**：出现 "API rate limit exceeded" 错误。

**解决方案**：
```bash
# 创建 GitHub 个人访问令牌
# https://github.com/settings/tokens

# 将其设置为环境变量
export GITHUB_TOKEN=ghp_your_token_here

# 添加到您的 shell 配置文件以持久化
echo 'export GITHUB_TOKEN=ghp_your_token_here' >> ~/.zshrc
source ~/.zshrc

# 验证它已设置
echo $GITHUB_TOKEN
```

### 身份验证失败

**问题**：出现 "Authentication required" 或 403 错误。

**解决方案**：
```bash
# 对于私有仓库，设置 GitHub 令牌
export GITHUB_TOKEN=your_token_with_repo_access

# 检查 Git 凭据
git config --global user.name
git config --global user.email

# 如果需要，更新 Git 凭据
git config --global credential.helper cache
```

---

## 常规调试

### 启用详细日志记录
默认情况下，大多数操作消息（扫描、更新、搜索）都被隐藏以保持输出整洁。
要查看详细的日志以进行调试，请将日志级别设置为 `DEBUG`：

```bash
# 使用详细输出运行命令
ask --log-level debug skill install browser-use
```

### 检查系统要求

```bash
# 验证所有要求
go version      # 应该是 1.25+
git --version   # 应该是 2.25+
which ask       # 应该显示安装路径

# 检查环境
env | grep GITHUB
```

### 常见环境变量

```bash
# 设置用于 API 访问的 GitHub 令牌（也支持 GH_TOKEN）
export GITHUB_TOKEN=your_token
```

---

## 获取帮助

如果您仍然遇到问题：

1. **检查现有问题**: https://github.com/yeasy/ask/issues
2. **搜索讨论**: https://github.com/yeasy/ask/discussions
3. **打开新问题**: 提供：
   - ASK 版本 (`ask version`)
   - 操作系统
   - Go 版本 (`go version`)
   - Git 版本 (`git --version`)
   - 完整的错误消息
   - 重现步骤

4. **社区支持**:
   - GitHub Discussions
   - Stack Overflow (tag: `ask-cli`)

---

## 快速参考

### 重置所有

```bash
# 完全重置 (小心！)
rm -rf .agent/skills
rm ask.yaml
rm ask.lock
ask init
```

### 验证安装

```bash
# 检查 ASK 是否工作
ask --help
ask skill search browser
ask repo list
```

### 常用工作流

```bash
# 1. 初始化项目
ask init

# 2. 搜索技能
ask skill search <keyword>

# 3. 安装技能
ask skill install <skill-name>

# 4. 验证安装
ask skill list

# 5. 保持技能更新
ask skill outdated
ask skill update
```
