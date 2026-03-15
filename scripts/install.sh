#!/usr/bin/env bash
# =============================================================================
# ProdOps Chronicles — Install Script
# =============================================================================
# Supports: Ubuntu 20.04, 22.04, 24.04
# Run as: sudo ./scripts/install.sh
# =============================================================================

set -euo pipefail

# ── Colours ───────────────────────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
RESET='\033[0m'

# ── Helpers ───────────────────────────────────────────────────────────────────
info()    { echo -e "${CYAN}[INFO]${RESET}  $*"; }
success() { echo -e "${GREEN}[OK]${RESET}    $*"; }
warn()    { echo -e "${YELLOW}[WARN]${RESET}  $*"; }
error()   { echo -e "${RED}[ERROR]${RESET} $*" >&2; }
die()     { error "$*"; exit 1; }

banner() {
  echo -e "${BOLD}${CYAN}"
  echo "  ██████╗ ██████╗  ██████╗ ██████╗  ██████╗ ██████╗ ███████╗"
  echo "  ██╔══██╗██╔══██╗██╔═══██╗██╔══██╗██╔═══██╗██╔══██╗██╔════╝"
  echo "  ██████╔╝██████╔╝██║   ██║██║  ██║██║   ██║██████╔╝███████╗"
  echo "  ██╔═══╝ ██╔══██╗██║   ██║██║  ██║██║   ██║██╔═══╝ ╚════██║"
  echo "  ██║     ██║  ██║╚██████╔╝██████╔╝╚██████╔╝██║     ███████║"
  echo "  ╚═╝     ╚═╝  ╚═╝ ╚═════╝ ╚═════╝  ╚═════╝ ╚═╝     ╚══════╝"
  echo -e "${RESET}"
  echo -e "${BOLD}  ProdOps Chronicles — Installer v0.1${RESET}"
  echo -e "  A self-hosted, hands-on DevOps learning platform."
  echo ""
}

# ── Constants ─────────────────────────────────────────────────────────────────
PRODOPS_BASE_DIR="/opt/prodops"
PRODOPS_PGDATA_DIR="${PRODOPS_BASE_DIR}/pgdata"
PRODOPS_MODULES_DIR="${PRODOPS_BASE_DIR}/modules"
PRODOPS_LOGS_DIR="${PRODOPS_BASE_DIR}/logs"
PRODOPS_BACKEND_DIR="${PRODOPS_BASE_DIR}/backend"
PRODOPS_USER="prodops"
PRODOPS_UID=1000
POSTGRES_USER="postgres"
POSTGRES_UID=999
POSTGRES_GID=999
INSTALL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# ── Root check ────────────────────────────────────────────────────────────────
check_root() {
  if [[ "${EUID}" -ne 0 ]]; then
    die "This script must be run as root. Try: sudo ./scripts/install.sh"
  fi
}

# ── OS check ──────────────────────────────────────────────────────────────────
check_os() {
  info "Checking operating system..."

  if [[ ! -f /etc/os-release ]]; then
    die "Cannot detect OS. /etc/os-release not found."
  fi

  # shellcheck source=/dev/null
  source /etc/os-release

  if [[ "${ID}" != "ubuntu" ]]; then
    die "Unsupported OS: ${ID}. ProdOps Chronicles currently supports Ubuntu only."
  fi

  case "${VERSION_ID}" in
    "20.04"|"22.04"|"24.04")
      success "Ubuntu ${VERSION_ID} detected — supported."
      ;;
    *)
      warn "Ubuntu ${VERSION_ID} is not officially tested. Proceeding anyway — things may break."
      ;;
  esac
}

# ── Architecture check ────────────────────────────────────────────────────────
check_arch() {
  info "Checking system architecture..."
  local arch
  arch="$(uname -m)"
  if [[ "${arch}" != "x86_64" ]]; then
    die "Unsupported architecture: ${arch}. Only x86_64 is supported in v1.0."
  fi
  success "Architecture: ${arch}"
}

# ── RAM check ─────────────────────────────────────────────────────────────────
check_ram() {
  info "Checking available RAM..."
  local ram_kb ram_gb
  ram_kb=$(grep MemTotal /proc/meminfo | awk '{print $2}')
  ram_gb=$(( ram_kb / 1024 / 1024 ))

  if [[ "${ram_gb}" -lt 3 ]]; then
    die "Insufficient RAM: ${ram_gb}GB detected. Minimum 4GB required."
  elif [[ "${ram_gb}" -lt 7 ]]; then
    warn "RAM: ${ram_gb}GB detected. 8GB recommended for the best experience."
  else
    success "RAM: ${ram_gb}GB — sufficient."
  fi
}

# ── Disk check ────────────────────────────────────────────────────────────────
check_disk() {
  info "Checking available disk space..."
  local free_kb free_gb
  free_kb=$(df / --output=avail | tail -1)
  free_gb=$(( free_kb / 1024 / 1024 ))

  if [[ "${free_gb}" -lt 10 ]]; then
    die "Insufficient disk space: ${free_gb}GB free. Minimum 10GB required."
  fi
  success "Disk: ${free_gb}GB free — sufficient."
}

# ── Internet check ────────────────────────────────────────────────────────────
check_internet() {
  info "Checking internet connectivity..."
  if ! curl -s --max-time 5 https://google.com > /dev/null 2>&1; then
    die "No internet connection detected. ProdOps Chronicles requires internet access to install dependencies."
  fi
  success "Internet connection available."
}

# ── Linux basics walkthrough ──────────────────────────────────────────────────
linux_basics_walkthrough() {
  echo ""
  echo -e "${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}"
  echo -e "${BOLD}  Quick Linux Primer${RESET}"
  echo -e "${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}"
  echo ""
  echo -e "  Before we install anything, here are the commands"
  echo -e "  this script uses under the hood. You will use these"
  echo -e "  every day as a DevOps engineer."
  echo ""
  echo -e "  ${CYAN}pwd${RESET}              → print working directory (where am I?)"
  echo -e "  ${CYAN}ls -la${RESET}           → list files including hidden ones"
  echo -e "  ${CYAN}mkdir -p <path>${RESET}  → create directory and all parents"
  echo -e "  ${CYAN}cat <file>${RESET}       → print file contents to terminal"
  echo -e "  ${CYAN}chmod <n> <file>${RESET} → change file permissions"
  echo -e "  ${CYAN}chown <u> <path>${RESET} → change file owner"
  echo -e "  ${CYAN}id <user>${RESET}        → show user UID and GID"
  echo -e "  ${CYAN}useradd${RESET}          → create a system user"
  echo -e "  ${CYAN}apt-get install${RESET}  → install a package (Ubuntu)"
  echo -e "  ${CYAN}curl <url>${RESET}       → fetch content from a URL"
  echo -e "  ${CYAN}tee <file>${RESET}       → write stdin to a file and stdout"
  echo ""
  echo -e "  ${YELLOW}Tip:${RESET} Run ${CYAN}man <command>${RESET} to read the manual for any command."
  echo -e "       Example: ${CYAN}man ls${RESET}"
  echo ""
  echo -e "${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}"
  echo ""

  read -rp "  Press ENTER to continue with the installation... "
  echo ""
}

# ── apt update ────────────────────────────────────────────────────────────────
update_apt() {
  info "Updating package lists..."
  apt-get update -qq
  success "Package lists updated."
}

# ── Install core dependencies ─────────────────────────────────────────────────
install_dependencies() {
  info "Installing core dependencies..."

  local packages=(
    curl
    wget
    git
    ca-certificates
    gnupg
    lsb-release
    apt-transport-https
    software-properties-common
  )

  for pkg in "${packages[@]}"; do
    if dpkg -l "${pkg}" &>/dev/null; then
      success "${pkg} already installed — skipping."
    else
      info "Installing ${pkg}..."
      apt-get install -y -qq "${pkg}"
      success "${pkg} installed."
    fi
  done
}

# ── Install Docker ────────────────────────────────────────────────────────────
install_docker() {
  info "Checking Docker installation..."

  if command -v docker &>/dev/null; then
    local version
    version=$(docker --version | awk '{print $3}' | tr -d ',')
    success "Docker already installed — version ${version}."
    return
  fi

  info "Installing Docker..."

  # Add Docker's official GPG key
  install -m 0755 -d /etc/apt/keyrings
  curl -fsSL https://download.docker.com/linux/ubuntu/gpg \
    | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
  chmod a+r /etc/apt/keyrings/docker.gpg

  # Add Docker repository
  echo \
    "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] \
    https://download.docker.com/linux/ubuntu \
    $(. /etc/os-release && echo "${VERSION_CODENAME}") stable" \
    | tee /etc/apt/sources.list.d/docker.list > /dev/null

  apt-get update -qq
  apt-get install -y -qq \
    docker-ce \
    docker-ce-cli \
    containerd.io \
    docker-buildx-plugin \
    docker-compose-plugin

  # Start and enable Docker
  systemctl enable docker --quiet
  systemctl start docker

  success "Docker installed successfully."
}

# ── Install Docker Compose (standalone) ──────────────────────────────────────
install_docker_compose() {
  info "Checking Docker Compose installation..."

  if command -v docker-compose &>/dev/null; then
    local version
    version=$(docker-compose --version | awk '{print $4}')
    success "Docker Compose already installed — version ${version}."
    return
  fi

  # docker compose plugin is installed with docker above
  # create a symlink for legacy docker-compose command
  if docker compose version &>/dev/null; then
    ln -sf /usr/libexec/docker/cli-plugins/docker-compose /usr/local/bin/docker-compose
    success "Docker Compose (plugin) symlinked as docker-compose."
    return
  fi

  die "Docker Compose installation failed. Please install manually."
}

# ── Add user to docker group ──────────────────────────────────────────────────
configure_docker_group() {
  info "Configuring Docker group..."

  # Get the sudo user (the user who ran sudo)
  local real_user="${SUDO_USER:-}"

  if [[ -z "${real_user}" ]]; then
    warn "Could not determine the non-root user. You may need to run: sudo usermod -aG docker \$USER"
    return
  fi

  if groups "${real_user}" | grep -q docker; then
    success "User '${real_user}' is already in the docker group."
  else
    usermod -aG docker "${real_user}"
    success "User '${real_user}' added to the docker group."
    warn "You will need to log out and back in for Docker group membership to take effect."
    warn "Or run: newgrp docker"
  fi
}

# ── Create postgres system user ───────────────────────────────────────────────
create_postgres_user() {
  info "Setting up postgres system user (UID ${POSTGRES_UID})..."

  # Check if UID 999 is already taken by a different user
  local existing_user
  existing_user=$(getent passwd "${POSTGRES_UID}" | cut -d: -f1 || true)

  if [[ -n "${existing_user}" && "${existing_user}" != "${POSTGRES_USER}" ]]; then
    die "UID ${POSTGRES_UID} is already assigned to '${existing_user}' on this system. \
Cannot create postgres user. Please update postgres_uid in base_configs.yaml \
and re-run the installer."
  fi

  # Check if postgres group exists
  if ! getent group "${POSTGRES_GID}" &>/dev/null; then
    groupadd -r -g "${POSTGRES_GID}" "${POSTGRES_USER}"
    success "Created postgres group (GID ${POSTGRES_GID})."
  else
    success "Postgres group (GID ${POSTGRES_GID}) already exists."
  fi

  # Check if postgres user exists
  if id "${POSTGRES_USER}" &>/dev/null; then
    success "Postgres system user already exists — skipping creation."
  else
    useradd \
      --system \
      --no-create-home \
      --uid "${POSTGRES_UID}" \
      --gid "${POSTGRES_GID}" \
      --shell /bin/false \
      "${POSTGRES_USER}"
    success "Postgres system user created (UID ${POSTGRES_UID})."
  fi
}

# ── Create prodops system user ────────────────────────────────────────────────
create_prodops_user() {
  info "Setting up prodops system user (UID ${PRODOPS_UID})..."

  local existing_user
  existing_user=$(getent passwd "${PRODOPS_UID}" | cut -d: -f1 || true)

  if [[ -n "${existing_user}" && "${existing_user}" != "${PRODOPS_USER}" ]]; then
    warn "UID ${PRODOPS_UID} is already taken by '${existing_user}'. Using current user instead."
    return
  fi

  if id "${PRODOPS_USER}" &>/dev/null; then
    success "prodops user already exists — skipping."
  else
    useradd \
      --system \
      --no-create-home \
      --uid "${PRODOPS_UID}" \
      --shell /bin/false \
      "${PRODOPS_USER}"
    success "prodops system user created (UID ${PRODOPS_UID})."
  fi
}

# ── Create directory structure ────────────────────────────────────────────────
create_directories() {
  info "Creating ProdOps directory structure under ${PRODOPS_BASE_DIR}..."

  local dirs=(
    "${PRODOPS_BASE_DIR}"
    "${PRODOPS_PGDATA_DIR}"
    "${PRODOPS_MODULES_DIR}"
    "${PRODOPS_LOGS_DIR}"
    "${PRODOPS_BACKEND_DIR}"
  )

  for dir in "${dirs[@]}"; do
    if [[ -d "${dir}" ]]; then
      success "Directory exists: ${dir}"
    else
      mkdir -p "${dir}"
      success "Created: ${dir}"
    fi
  done

  # Set ownership
  # pgdata must be owned by postgres (UID 999) for the container to start
  chown -R "${POSTGRES_UID}:${POSTGRES_GID}" "${PRODOPS_PGDATA_DIR}"
  success "Set ownership of ${PRODOPS_PGDATA_DIR} to postgres (${POSTGRES_UID}:${POSTGRES_GID})."

  # Other dirs owned by prodops user
  chown -R "${PRODOPS_UID}:${PRODOPS_UID}" \
    "${PRODOPS_MODULES_DIR}" \
    "${PRODOPS_LOGS_DIR}" \
    "${PRODOPS_BACKEND_DIR}"

  # Base dir readable by all
  chmod 755 "${PRODOPS_BASE_DIR}"
  success "Directory structure ready."
}

# ── Generate base_configs.yaml ────────────────────────────────────────────────
generate_base_configs() {
  local config_file="${INSTALL_DIR}/base_configs.yaml"

  if [[ -f "${config_file}" ]]; then
    warn "base_configs.yaml already exists — skipping generation."
    warn "Delete it and re-run the installer to regenerate."
    return
  fi

  info "Generating base_configs.yaml..."

  cat > "${config_file}" << EOF
# =============================================================================
# base_configs.yaml — ProdOps Chronicles
# Generated by install.sh on $(date '+%Y-%m-%d %H:%M:%S')
# DO NOT commit this file — it is in .gitignore
# To regenerate: delete this file and re-run ./scripts/install.sh
# Edit this file to change difficulty, enable/disable modules, or override paths.
# =============================================================================

system:
  postgres_uid: ${POSTGRES_UID}
  postgres_gid: ${POSTGRES_GID}
  prodops_user: ${PRODOPS_USER}
  prodops_uid:  ${PRODOPS_UID}
  prodops_gid:  ${PRODOPS_UID}

storage:
  base_path:    ${PRODOPS_BASE_DIR}
  pgdata_path:  ${PRODOPS_PGDATA_DIR}
  modules_path: ${PRODOPS_MODULES_DIR}
  sync_path:    ${PRODOPS_BASE_DIR}/sync
  logs_path:    ${PRODOPS_LOGS_DIR}
  backend_path: ${PRODOPS_BACKEND_DIR}

versions:
  postgres:   "15.4"
  backend:    "0.1.0"
  gitea:      "1.21"
  woodpecker: "2.3"

network:
  prodops_subnet: 10.42.0.0/16
  dns_domain:     prodops.local

runtime: compose

# Difficulty level. Controls hint availability and which modules are visible.
# d1 = DevOps Engineer        (all hints, all beginner modules)
# d2 = Senior DevOps Engineer (2 hints, intermediate modules unlocked)
# d3 = DevOps Team Lead       (1 hint, all modules visible)
difficulty: d1

# Module availability. Set enabled: true to make a module available.
# min_difficulty controls which player levels can see the module.
modules:
  linux-cli:
    enabled: true
    min_difficulty: d1
    port: null

  git:
    enabled: true
    min_difficulty: d1
    port: null

  bash-scripting:
    enabled: true
    min_difficulty: d1
    port: null

  docker:
    enabled: true
    min_difficulty: d1
    port: null

  docker-compose:
    enabled: true
    min_difficulty: d1
    port: null

  cicd:
    enabled: true
    min_difficulty: d1
    port: 8100

  terraform:
    enabled: false
    min_difficulty: d1
    port: null

  go:
    enabled: false
    min_difficulty: d1
    port: null

  devsecops:
    enabled: false
    min_difficulty: d2
    port: null

  kubernetes:
    enabled: false
    min_difficulty: d1
    port: null

  traefik:
    enabled: false
    min_difficulty: d1
    port: null

  jenkins:
    enabled: false
    min_difficulty: d2
    port: 8080

  ansible:
    enabled: false
    min_difficulty: d2
    port: null

  aws:
    enabled: false
    min_difficulty: d2
    port: null

  prometheus:
    enabled: false
    min_difficulty: d2
    port: 9090

  grafana:
    enabled: false
    min_difficulty: d2
    port: 3000

  service-mesh:
    enabled: false
    min_difficulty: d3
    port: null

ai:
  provider: ollama
  api_key:  ""

telemetry:
  enabled: false
  level:   tier1

EOF

  success "base_configs.yaml generated at ${config_file}."
}

# ── Generate values.yaml ──────────────────────────────────────────────────────
generate_values() {
  local values_file="${INSTALL_DIR}/values.yaml"

  if [[ -f "${values_file}" ]]; then
    warn "values.yaml already exists — skipping generation."
    warn "Delete it and re-run the installer to regenerate."
    return
  fi

  info "Generating values.yaml..."

  # Canonical module/difficulty config lives in base_configs.yaml.
  # This file documents the v1.0 module unlock order for reference.
  cat > "${values_file}" << 'VALEOF'
# =============================================================================
# values.yaml — ProdOps Chronicles
# NOTE: Module and difficulty configuration has moved to base_configs.yaml.
# Edit base_configs.yaml to enable/disable modules and change difficulty.
# This file documents the v1.0 module unlock order only.
# =============================================================================

# Module unlock order for v1.0 (beginner, Docker Compose runtime).
# The backend enforces this order via the requires_module_id DB field.
module_order:
  - linux-cli        # Module 1 — first module, unlocked by default
  - git              # Module 2 — unlocks after linux-cli
  - bash-scripting   # Module 3 — unlocks after git
  - docker           # Module 4 — unlocks after bash-scripting
  - docker-compose   # Module 5 — unlocks after docker
  - cicd             # Module 6 — unlocks after docker-compose
VALEOF

  success "values.yaml generated at ${values_file}."
}

# ── Verify installations ──────────────────────────────────────────────────────
verify_installations() {
  echo ""
  info "Verifying installations..."

  local all_ok=true

  check_cmd() {
    local cmd="$1"
    local label="$2"
    if command -v "${cmd}" &>/dev/null; then
      local version
      version=$("${cmd}" --version 2>&1 | head -1)
      success "${label}: ${version}"
    else
      error "${label}: NOT FOUND"
      all_ok=false
    fi
  }

  check_cmd docker    "Docker"
  check_cmd git       "Git"
  check_cmd curl      "curl"

  # docker compose
  if docker compose version &>/dev/null; then
    success "Docker Compose: $(docker compose version --short)"
  else
    error "Docker Compose: NOT FOUND"
    all_ok=false
  fi

  if [[ "${all_ok}" == "false" ]]; then
    die "One or more installations failed. Check the errors above."
  fi
}

# ── Summary ───────────────────────────────────────────────────────────────────
print_summary() {
  echo ""
  echo -e "${BOLD}${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}"
  echo -e "${BOLD}${GREEN}  Installation Complete!${RESET}"
  echo -e "${BOLD}${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}"
  echo ""
  echo -e "  ${BOLD}What was set up:${RESET}"
  echo -e "  ✓  Core dependencies (curl, wget, git)"
  echo -e "  ✓  Docker + Docker Compose"
  echo -e "  ✓  postgres system user (UID ${POSTGRES_UID})"
  echo -e "  ✓  prodops system user (UID ${PRODOPS_UID})"
  echo -e "  ✓  Directory structure at ${PRODOPS_BASE_DIR}"
  echo -e "  ✓  base_configs.yaml generated"
  echo -e "  ✓  values.yaml generated"
  echo ""
  echo -e "  ${BOLD}Next steps:${RESET}"
  echo -e "  1. ${YELLOW}Log out and back in${RESET} (or run ${CYAN}newgrp docker${RESET}) to use Docker without sudo"
  echo -e "  2. Build and start ProdOps: ${CYAN}prodops start${RESET}"
  echo -e "     ${YELLOW}(prodops CLI coming in dev/v0.3)${RESET}"
  echo -e "  3. Enable your first module: ${CYAN}prodops module enable git${RESET}"
  echo ""
  echo -e "  ${BOLD}Useful paths:${RESET}"
  echo -e "  Data directory:   ${CYAN}${PRODOPS_BASE_DIR}${RESET}"
  echo -e "  Postgres data:    ${CYAN}${PRODOPS_PGDATA_DIR}${RESET}"
  echo -e "  Config:           ${CYAN}${INSTALL_DIR}/base_configs.yaml${RESET}"
  echo -e "  Module config:    ${CYAN}${INSTALL_DIR}/values.yaml${RESET}"
  echo ""
  echo -e "  ${BOLD}Docs:${RESET} https://github.com/<your-username>/prodops-chronicles"
  echo ""
}

# ── Main ──────────────────────────────────────────────────────────────────────
main() {
  banner

  check_root
  check_os
  check_arch
  check_ram
  check_disk
  check_internet

  linux_basics_walkthrough

  echo -e "${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}"
  echo -e "${BOLD}  Starting Installation${RESET}"
  echo -e "${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}"
  echo ""

  update_apt
  install_dependencies
  install_docker
  install_docker_compose
  configure_docker_group
  create_postgres_user
  create_prodops_user
  create_directories
  generate_base_configs
  generate_values
  verify_installations
  print_summary
}

main "$@"
