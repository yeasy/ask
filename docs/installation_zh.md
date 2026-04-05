# 安装指南

本指南涵盖了在您的系统上安装 ASK 的所有方法。

## 系统要求

- **操作系统**: macOS, Linux, 或 Windows
- **Go** (可选): 1.25+ 如果从源代码构建

---

## macOS (Homebrew)

在 macOS 上安装 ASK 最简单的方法：

```bash
brew tap yeasy/tap
brew install ask
```

更新：

```bash
brew upgrade ask
```

---

## Linux

### 下载二进制文件

下载适合您架构的最新版本：

```bash
# AMD64
curl -LO https://github.com/yeasy/ask/releases/latest/download/ask_linux_amd64.tar.gz
tar xzf ask_linux_amd64.tar.gz
sudo mv ask /usr/local/bin/

# ARM64
curl -LO https://github.com/yeasy/ask/releases/latest/download/ask_linux_arm64.tar.gz
tar xzf ask_linux_arm64.tar.gz
sudo mv ask /usr/local/bin/
```

### 验证安装

```bash
ask version
```

---

## Windows

### 下载二进制文件

1. 从 [Releases](https://github.com/yeasy/ask/releases) 下载 `ask_windows_amd64.zip`
2. 解压 zip 文件
3. 将目录添加到您的 PATH

### 使用 PowerShell

```powershell
# 下载并解压
Invoke-WebRequest -Uri "https://github.com/yeasy/ask/releases/latest/download/ask_windows_amd64.zip" -OutFile "ask.zip"
Expand-Archive -Path "ask.zip" -DestinationPath "C:\tools\ask"

# 添加到 PATH (以管理员身份运行)
[Environment]::SetEnvironmentVariable("Path", $env:Path + ";C:\tools\ask", "Machine")
```

---

## 从源代码构建

### 前置条件

- **Go**: 版本 1.25 或更高。
- **Wails**: 构建桌面应用程序所需。
- **Node.js**: 构建前端组件所需。

### 1. 构建 CLI 工具

构建命令行界面 (CLI) 工具：

```bash
# 克隆仓库
git clone https://github.com/yeasy/ask.git
cd ask

# 安装依赖并构建
make build

# 移动二进制文件到路径 (macOS/Linux)
sudo mv ask /usr/local/bin/
```

或者直接使用 `go install` 安装：

```bash
go install github.com/yeasy/ask@latest
```

### 2. 构建桌面应用程序

桌面应用程序使用 [Wails](https://wails.io) 构建。

#### 步骤 1: 安装 Wails

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

#### 步骤 2: 构建应用

在项目根目录下运行以下命令：

```bash
make build-desktop
```

这将在 `build/bin/` 目录中生成应用程序。

#### 平台特定说明

**macOS:**
- 输出将是 `build/bin/ask-desktop.app`。
- 如果遇到 "unidentified developer" 警告，请转到 **系统设置 > 隐私与安全性** 并允许应用运行。
- 要构建 `.dmg` (需要 `create-dmg`)：
  ```bash
  wails build -platform darwin/universal
  ```

**Windows:**
- 输出将是 `build/bin/ask-desktop.exe`。
- 确保已安装 WebView2 运行时 (Windows 10/11 标配)。

**Linux:**
- 输出将是 `build/bin/ask-desktop`。
- 您可能需要安装 GTK3 和 WebKit2GTK 开发头文件：
  ```bash
  # Debian/Ubuntu
  sudo apt install libgtk-3-dev libwebkit2gtk-4.0-dev
  
  # Fedora
  sudo dnf install gtk3-devel webkit2gtk3-devel
  ```

---

## 验证安装

安装后，验证 ASK 是否正常工作：

```bash
ask version
ask --help
```

## 下一步

1. [初始化您的第一个项目](commands_zh.md#ask-init)
2. [搜索技能](commands_zh.md#ask-skill-search)
3. [安装技能](commands_zh.md#ask-skill-install)
