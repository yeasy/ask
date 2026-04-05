# Installation Guide

This guide covers all methods of installing ASK on your system.

## Requirements

- **Operating System**: macOS, Linux, or Windows
- **Go** (optional): 1.25+ if building from source

---

## macOS (Homebrew)

The easiest way to install ASK on macOS:

```bash
brew tap yeasy/tap
brew install ask
```

To update:

```bash
brew upgrade ask
```

---

## Linux

### Download Binary

Download the latest release for your architecture:

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

### Verify Installation

```bash
ask version
```

---

## Windows

### Download Binary

1. Download `ask_windows_amd64.zip` from [Releases](https://github.com/yeasy/ask/releases)
2. Extract the zip file
3. Add the directory to your PATH

### Using PowerShell

```powershell
# Download and extract
Invoke-WebRequest -Uri "https://github.com/yeasy/ask/releases/latest/download/ask_windows_amd64.zip" -OutFile "ask.zip"
Expand-Archive -Path "ask.zip" -DestinationPath "C:\tools\ask"

# Add to PATH (run as Administrator)
[Environment]::SetEnvironmentVariable("Path", $env:Path + ";C:\tools\ask", "Machine")
```

---

## Build from Source

### Prerequisites

- **Go**: Version 1.25 or higher.
- **Wails**: Required for building the desktop application.
- **Node.js**: Required for building the frontend components.

### 1. Build CLI Tool

To build the command-line interface (CLI) tool:

```bash
# Clone the repository
git clone https://github.com/yeasy/ask.git
cd ask

# Install dependencies and build
make build

# Move binary to path (macOS/Linux)
sudo mv ask /usr/local/bin/
```

Or install directly using `go install`:

```bash
go install github.com/yeasy/ask@latest
```

### 2. Build Desktop Application

The desktop application is built using [Wails](https://wails.io).

#### Step 1: Install Wails

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

#### Step 2: Build App

Run the following command in the project root:

```bash
make build-desktop
```

This will generate the application in the `build/bin/` directory.

#### Platform Specifics

**macOS:**
- The output will be `build/bin/ask-desktop.app`.
- If you encounter an "unidentified developer" warning, go to **System Settings > Privacy & Security** and allow the app to run.
- To build a `.dmg` (requires `create-dmg`):
  ```bash
  wails build -platform darwin/universal
  ```

**Windows:**
- The output will be `build/bin/ask-desktop.exe`.
- Ensure you have the WebView2 runtime installed (standard on Windows 10/11).

**Linux:**
- The output will be `build/bin/ask-desktop`.
- You may need to install GTK3 and WebKit2GTK development headers:
  ```bash
  # Debian/Ubuntu
  sudo apt install libgtk-3-dev libwebkit2gtk-4.0-dev
  
  # Fedora
  sudo dnf install gtk3-devel webkit2gtk3-devel
  ```


---

## Verify Installation

After installation, verify ASK is working:

```bash
ask version
ask --help
```

## Next Steps

1. [Initialize your first project](commands.md#ask-init)
2. [Search for skills](commands.md#ask-skill-search)
3. [Install a skill](commands.md#ask-skill-install)
