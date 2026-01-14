#!/bin/bash
# youtube-rtsp-proxy installation script
# Supports system-wide installation (sudo) and user-local installation (no sudo)

set -e

# Version
SCRIPT_VERSION="1.0.0"
BINARY_NAME="youtube-rtsp-proxy"

# Default paths (system-wide installation)
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/youtube-rtsp-proxy"
DATA_DIR="/var/lib/youtube-rtsp-proxy"

# Flags
USER_MODE=false
INSTALL_DEPS=false
DRY_RUN=false
PREFIX=""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# ============================================================
# Helper Functions
# ============================================================

echo_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

echo_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

echo_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

echo_step() {
    echo -e "${BLUE}[STEP]${NC} $1"
}

show_help() {
    cat << EOF
YouTube to RTSP Proxy Installer v${SCRIPT_VERSION}

Usage: ./install.sh [OPTIONS]

Options:
  --user              Install to user directory (~/.local/bin)
                      No sudo required
  --prefix <path>     Install to custom directory
                      Example: --prefix ~/mytools
  --install-deps      Automatically install missing dependencies
                      (ffmpeg, yt-dlp, mediamtx)
  --dry-run           Show what would be done without making changes
  -h, --help          Show this help message

Examples:
  # System-wide installation (requires sudo)
  sudo ./install.sh

  # System-wide with automatic dependency installation
  sudo ./install.sh --install-deps

  # User-local installation (no sudo required)
  ./install.sh --user

  # User-local with automatic dependencies
  ./install.sh --user --install-deps

  # Custom prefix installation
  ./install.sh --prefix ~/tools --install-deps

EOF
}

# ============================================================
# Argument Parsing
# ============================================================

parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --user)
                USER_MODE=true
                shift
                ;;
            --prefix)
                PREFIX="$2"
                shift 2
                ;;
            --install-deps)
                INSTALL_DEPS=true
                shift
                ;;
            --dry-run)
                DRY_RUN=true
                shift
                ;;
            -h|--help)
                show_help
                exit 0
                ;;
            *)
                echo_error "Unknown option: $1"
                echo "Use --help for usage information"
                exit 1
                ;;
        esac
    done
}

# ============================================================
# Path Configuration
# ============================================================

configure_paths() {
    if [ -n "$PREFIX" ]; then
        # Custom prefix mode
        INSTALL_DIR="${PREFIX}/bin"
        CONFIG_DIR="${PREFIX}/etc/youtube-rtsp-proxy"
        DATA_DIR="${PREFIX}/share/youtube-rtsp-proxy"
        USER_MODE=true  # Custom prefix implies user mode (no sudo)
    elif [ "$USER_MODE" = true ]; then
        # User mode (~/.local)
        INSTALL_DIR="$HOME/.local/bin"
        CONFIG_DIR="$HOME/.config/youtube-rtsp-proxy"
        DATA_DIR="$HOME/.local/share/youtube-rtsp-proxy"
    fi
    # else: use default system paths
}

# ============================================================
# Permission Check
# ============================================================

check_permissions() {
    if [ "$USER_MODE" = false ] && [ "$EUID" -ne 0 ]; then
        echo_error "System-wide installation requires root privileges."
        echo ""
        echo "Options:"
        echo "  1. Run with sudo:        sudo ./install.sh"
        echo "  2. Install for user:     ./install.sh --user"
        echo "  3. Custom prefix:        ./install.sh --prefix ~/mytools"
        echo ""
        exit 1
    fi
}

# ============================================================
# OS and Architecture Detection
# ============================================================

detect_system() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    # Normalize architecture names
    case $ARCH in
        x86_64|amd64)
            ARCH="amd64"
            ;;
        aarch64|arm64)
            ARCH="arm64"
            ;;
        armv7l|armhf)
            ARCH="armv7"
            ;;
        *)
            echo_warn "Unknown architecture: $ARCH"
            ;;
    esac

    # Detect package manager
    PKG_MANAGER=""
    if command -v apt-get &>/dev/null; then
        PKG_MANAGER="apt"
    elif command -v dnf &>/dev/null; then
        PKG_MANAGER="dnf"
    elif command -v yum &>/dev/null; then
        PKG_MANAGER="yum"
    elif command -v pacman &>/dev/null; then
        PKG_MANAGER="pacman"
    elif command -v brew &>/dev/null; then
        PKG_MANAGER="brew"
    elif command -v apk &>/dev/null; then
        PKG_MANAGER="apk"
    fi

    echo_info "Detected: OS=$OS, ARCH=$ARCH, PKG_MANAGER=$PKG_MANAGER"
}

# ============================================================
# Dependency Check Functions
# ============================================================

check_dependency() {
    local name=$1
    if command -v "$name" &>/dev/null; then
        return 0
    fi
    return 1
}

get_missing_dependencies() {
    local missing=()

    if ! check_dependency ffmpeg; then
        missing+=("ffmpeg")
    fi

    if ! check_dependency yt-dlp; then
        missing+=("yt-dlp")
    fi

    if ! check_dependency mediamtx; then
        missing+=("mediamtx")
    fi

    echo "${missing[@]}"
}

# ============================================================
# Dependency Installation Functions
# ============================================================

install_ffmpeg() {
    echo_step "Installing ffmpeg..."

    if [ "$DRY_RUN" = true ]; then
        echo "  [DRY-RUN] Would install ffmpeg via $PKG_MANAGER"
        return 0
    fi

    case $PKG_MANAGER in
        apt)
            if [ "$USER_MODE" = false ]; then
                apt-get update && apt-get install -y ffmpeg
            else
                echo_error "ffmpeg requires system package manager. Please run:"
                echo "  sudo apt-get install ffmpeg"
                return 1
            fi
            ;;
        dnf|yum)
            if [ "$USER_MODE" = false ]; then
                $PKG_MANAGER install -y ffmpeg
            else
                echo_error "ffmpeg requires system package manager. Please run:"
                echo "  sudo $PKG_MANAGER install ffmpeg"
                return 1
            fi
            ;;
        pacman)
            if [ "$USER_MODE" = false ]; then
                pacman -S --noconfirm ffmpeg
            else
                echo_error "ffmpeg requires system package manager. Please run:"
                echo "  sudo pacman -S ffmpeg"
                return 1
            fi
            ;;
        apk)
            if [ "$USER_MODE" = false ]; then
                apk add ffmpeg
            else
                echo_error "ffmpeg requires system package manager. Please run:"
                echo "  sudo apk add ffmpeg"
                return 1
            fi
            ;;
        brew)
            brew install ffmpeg
            ;;
        *)
            echo_error "Cannot auto-install ffmpeg. Please install manually:"
            echo "  Debian/Ubuntu: sudo apt install ffmpeg"
            echo "  Fedora:        sudo dnf install ffmpeg"
            echo "  Arch:          sudo pacman -S ffmpeg"
            echo "  macOS:         brew install ffmpeg"
            return 1
            ;;
    esac

    echo_info "ffmpeg installed successfully"
}

install_ytdlp() {
    echo_step "Installing yt-dlp..."

    if [ "$DRY_RUN" = true ]; then
        echo "  [DRY-RUN] Would install yt-dlp"
        return 0
    fi

    # Try pip first
    if command -v pip3 &>/dev/null; then
        if [ "$USER_MODE" = true ]; then
            pip3 install --user yt-dlp
        else
            pip3 install yt-dlp
        fi
        echo_info "yt-dlp installed via pip"
        return 0
    elif command -v pip &>/dev/null; then
        if [ "$USER_MODE" = true ]; then
            pip install --user yt-dlp
        else
            pip install yt-dlp
        fi
        echo_info "yt-dlp installed via pip"
        return 0
    fi

    # Fallback: download binary directly
    echo_info "pip not found, downloading yt-dlp binary..."

    local ytdlp_url="https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp"
    local target_path="${INSTALL_DIR}/yt-dlp"

    mkdir -p "$INSTALL_DIR"

    if command -v curl &>/dev/null; then
        curl -L "$ytdlp_url" -o "$target_path"
    elif command -v wget &>/dev/null; then
        wget -O "$target_path" "$ytdlp_url"
    else
        echo_error "Neither curl nor wget found. Cannot download yt-dlp."
        return 1
    fi

    chmod +x "$target_path"
    echo_info "yt-dlp installed to $target_path"
}

install_mediamtx() {
    echo_step "Installing mediamtx..."

    if [ "$DRY_RUN" = true ]; then
        echo "  [DRY-RUN] Would download and install mediamtx"
        return 0
    fi

    # Determine the correct release filename
    local os_name=""
    local arch_name=""

    case $OS in
        linux)
            os_name="linux"
            ;;
        darwin)
            os_name="darwin"
            ;;
        *)
            echo_error "Unsupported OS for mediamtx: $OS"
            return 1
            ;;
    esac

    case $ARCH in
        amd64)
            arch_name="amd64"
            ;;
        arm64)
            arch_name="arm64"
            ;;
        armv7)
            arch_name="armv7"
            ;;
        *)
            echo_error "Unsupported architecture for mediamtx: $ARCH"
            return 1
            ;;
    esac

    # Get latest release URL from GitHub API
    echo_info "Fetching latest mediamtx release..."

    local api_url="https://api.github.com/repos/bluenviron/mediamtx/releases/latest"
    local release_info=""

    if command -v curl &>/dev/null; then
        release_info=$(curl -s "$api_url")
    elif command -v wget &>/dev/null; then
        release_info=$(wget -qO- "$api_url")
    else
        echo_error "Neither curl nor wget found."
        return 1
    fi

    # Find the download URL for our OS/ARCH combination
    local download_url=""
    download_url=$(echo "$release_info" | grep -o "https://[^\"]*mediamtx_[^\"]*_${os_name}_${arch_name}\.tar\.gz" | head -1)

    if [ -z "$download_url" ]; then
        echo_error "Could not find mediamtx release for ${os_name}/${arch_name}"
        echo "Please download manually from: https://github.com/bluenviron/mediamtx/releases"
        return 1
    fi

    echo_info "Downloading from: $download_url"

    # Create temp directory
    local tmp_dir=$(mktemp -d)
    local archive_path="${tmp_dir}/mediamtx.tar.gz"

    # Download
    if command -v curl &>/dev/null; then
        curl -L "$download_url" -o "$archive_path"
    else
        wget -O "$archive_path" "$download_url"
    fi

    # Extract
    mkdir -p "$INSTALL_DIR"
    tar -xzf "$archive_path" -C "$tmp_dir"

    # Install binary
    if [ -f "${tmp_dir}/mediamtx" ]; then
        mv "${tmp_dir}/mediamtx" "${INSTALL_DIR}/mediamtx"
        chmod +x "${INSTALL_DIR}/mediamtx"
    else
        echo_error "mediamtx binary not found in archive"
        rm -rf "$tmp_dir"
        return 1
    fi

    # Cleanup
    rm -rf "$tmp_dir"

    echo_info "mediamtx installed to ${INSTALL_DIR}/mediamtx"
}

install_missing_dependencies() {
    local missing=($1)

    if [ ${#missing[@]} -eq 0 ]; then
        echo_info "All dependencies are already installed"
        return 0
    fi

    echo_step "Installing missing dependencies: ${missing[*]}"

    local failed=()

    for dep in "${missing[@]}"; do
        case $dep in
            ffmpeg)
                if ! install_ffmpeg; then
                    failed+=("ffmpeg")
                fi
                ;;
            yt-dlp)
                if ! install_ytdlp; then
                    failed+=("yt-dlp")
                fi
                ;;
            mediamtx)
                if ! install_mediamtx; then
                    failed+=("mediamtx")
                fi
                ;;
        esac
    done

    if [ ${#failed[@]} -gt 0 ]; then
        echo_error "Failed to install: ${failed[*]}"
        echo "Please install these dependencies manually."
        return 1
    fi

    return 0
}

# ============================================================
# PATH Setup (for user mode)
# ============================================================

setup_path() {
    if [ "$USER_MODE" = false ]; then
        return 0  # System paths should already be in PATH
    fi

    # Check if INSTALL_DIR is already in PATH
    if echo "$PATH" | grep -q "$INSTALL_DIR"; then
        echo_info "PATH already contains $INSTALL_DIR"
        return 0
    fi

    # Detect shell config file
    local shell_rc=""
    local shell_name=""

    if [ -n "$ZSH_VERSION" ] || [ -f "$HOME/.zshrc" ]; then
        shell_rc="$HOME/.zshrc"
        shell_name="zsh"
    elif [ -n "$FISH_VERSION" ] || [ -f "$HOME/.config/fish/config.fish" ]; then
        shell_rc="$HOME/.config/fish/config.fish"
        shell_name="fish"
    else
        shell_rc="$HOME/.bashrc"
        shell_name="bash"
    fi

    if [ "$DRY_RUN" = true ]; then
        echo "  [DRY-RUN] Would add PATH to $shell_rc"
        return 0
    fi

    # Check if already added
    if grep -q "youtube-rtsp-proxy" "$shell_rc" 2>/dev/null; then
        echo_info "PATH entry already exists in $shell_rc"
        return 0
    fi

    # Add PATH entry
    echo "" >> "$shell_rc"
    echo "# youtube-rtsp-proxy" >> "$shell_rc"

    if [ "$shell_name" = "fish" ]; then
        echo "set -gx PATH \$PATH $INSTALL_DIR" >> "$shell_rc"
    else
        echo "export PATH=\"\$PATH:$INSTALL_DIR\"" >> "$shell_rc"
    fi

    echo_info "PATH added to $shell_rc"
    echo ""
    echo -e "${YELLOW}========================================${NC}"
    echo -e "${YELLOW}  IMPORTANT: Run the following command  ${NC}"
    echo -e "${YELLOW}  to apply PATH changes:                ${NC}"
    echo -e "${YELLOW}========================================${NC}"
    echo ""
    echo -e "  ${GREEN}source $shell_rc${NC}"
    echo ""
}

# ============================================================
# Main Installation Functions
# ============================================================

create_directories() {
    echo_step "Creating directories..."

    if [ "$DRY_RUN" = true ]; then
        echo "  [DRY-RUN] Would create:"
        echo "    - $INSTALL_DIR"
        echo "    - $CONFIG_DIR"
        echo "    - $DATA_DIR"
        return 0
    fi

    mkdir -p "$INSTALL_DIR"
    mkdir -p "$CONFIG_DIR"
    mkdir -p "$DATA_DIR"

    echo_info "Directories created"
}

install_binary() {
    local binary_path="bin/$BINARY_NAME"

    # Try to find binary relative to script location
    local script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    local project_dir="$(dirname "$script_dir")"

    if [ -f "${project_dir}/bin/$BINARY_NAME" ]; then
        binary_path="${project_dir}/bin/$BINARY_NAME"
    elif [ -f "./bin/$BINARY_NAME" ]; then
        binary_path="./bin/$BINARY_NAME"
    fi

    if [ ! -f "$binary_path" ]; then
        echo_error "Binary not found: $binary_path"
        echo "Build first with: make build"
        exit 1
    fi

    echo_step "Installing binary..."

    if [ "$DRY_RUN" = true ]; then
        echo "  [DRY-RUN] Would copy $binary_path to $INSTALL_DIR/"
        return 0
    fi

    cp "$binary_path" "$INSTALL_DIR/"
    chmod +x "$INSTALL_DIR/$BINARY_NAME"

    echo_info "Binary installed to $INSTALL_DIR/$BINARY_NAME"
}

install_config() {
    local script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    local project_dir="$(dirname "$script_dir")"
    local config_src="${project_dir}/configs/config.example.yaml"
    local config_dest="$CONFIG_DIR/config.yaml"

    if [ ! -f "$config_src" ]; then
        config_src="./configs/config.example.yaml"
    fi

    echo_step "Installing config file..."

    if [ "$DRY_RUN" = true ]; then
        echo "  [DRY-RUN] Would copy config to $config_dest"
        return 0
    fi

    if [ -f "$config_dest" ]; then
        echo_warn "Config file already exists: $config_dest"
        echo_warn "Skipping to preserve existing settings."
    else
        if [ -f "$config_src" ]; then
            cp "$config_src" "$config_dest"
            echo_info "Config installed to $config_dest"
        else
            echo_warn "Config template not found, skipping"
        fi
    fi
}

# ============================================================
# Print Results
# ============================================================

print_success() {
    echo ""
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}  Installation Complete!                ${NC}"
    echo -e "${GREEN}========================================${NC}"
    echo ""
    echo "Installation paths:"
    echo "  Binary:  $INSTALL_DIR/$BINARY_NAME"
    echo "  Config:  $CONFIG_DIR/config.yaml"
    echo "  Data:    $DATA_DIR"
    echo ""
    echo "Quick start:"
    echo "  # Start the RTSP server"
    echo "  $BINARY_NAME server start"
    echo ""
    echo "  # Start proxying a YouTube stream"
    echo "  $BINARY_NAME start 'https://www.youtube.com/watch?v=...' --name mystream"
    echo ""
    echo "  # View the stream"
    echo "  ffplay rtsp://localhost:8554/mystream"
    echo ""

    if [ "$USER_MODE" = true ]; then
        echo -e "${YELLOW}Note: If the command is not found, run:${NC}"
        echo "  source ~/.bashrc  (or ~/.zshrc)"
        echo ""
    fi
}

print_dry_run_summary() {
    echo ""
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}  Dry Run Summary                       ${NC}"
    echo -e "${BLUE}========================================${NC}"
    echo ""
    echo "Mode: $([ "$USER_MODE" = true ] && echo "User" || echo "System")"
    echo ""
    echo "Would install to:"
    echo "  Binary:  $INSTALL_DIR/$BINARY_NAME"
    echo "  Config:  $CONFIG_DIR/config.yaml"
    echo "  Data:    $DATA_DIR"
    echo ""
    if [ "$INSTALL_DEPS" = true ]; then
        local missing=$(get_missing_dependencies)
        if [ -n "$missing" ]; then
            echo "Would install dependencies: $missing"
        else
            echo "All dependencies already installed"
        fi
    fi
    echo ""
}

# ============================================================
# Main
# ============================================================

main() {
    echo "=========================================="
    echo " YouTube to RTSP Proxy Installer"
    echo " Version: $SCRIPT_VERSION"
    echo "=========================================="
    echo ""

    # Parse command line arguments
    parse_args "$@"

    # Configure installation paths
    configure_paths

    # Detect system info
    detect_system

    # Check permissions (only for system-wide install)
    check_permissions

    # Handle dry run
    if [ "$DRY_RUN" = true ]; then
        print_dry_run_summary
        exit 0
    fi

    # Check and optionally install dependencies
    local missing=$(get_missing_dependencies)

    if [ -n "$missing" ]; then
        if [ "$INSTALL_DEPS" = true ]; then
            install_missing_dependencies "$missing" || exit 1
        else
            echo_error "Missing dependencies: $missing"
            echo ""
            echo "Options:"
            echo "  1. Auto-install: ./install.sh --install-deps"
            echo "  2. Manual install:"
            for dep in $missing; do
                case $dep in
                    ffmpeg)
                        echo "     ffmpeg:   sudo apt install ffmpeg"
                        ;;
                    yt-dlp)
                        echo "     yt-dlp:   pip install yt-dlp"
                        ;;
                    mediamtx)
                        echo "     mediamtx: https://github.com/bluenviron/mediamtx/releases"
                        ;;
                esac
            done
            echo ""
            exit 1
        fi
    else
        echo_info "All dependencies found"
    fi

    # Create directories
    create_directories

    # Install binary
    install_binary

    # Install config
    install_config

    # Setup PATH for user mode
    setup_path

    # Print success message
    print_success
}

# Run main with all arguments
main "$@"
