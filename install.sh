#!/usr/bin/env bash
# =============================================================================
# CASCATA V1 — INSTALADOR AUTOMÁTICO
# =============================================================================
# Pré-condição: Docker Engine + Docker Compose plugin já instalados.
#               (ver pre-feito.txt para instruções de Docker)
#
# O que este script faz:
#   1. Detecta o sistema operacional
#   2. Verifica Docker (obrigatório — aborta se ausente)
#   3. Instala Go 1.23+ (se ausente)
#   4. Instala Rust/Cargo (se ausente)
#   5. Instala Node.js 20+ via nvm (se ausente)
#   6. Clona o repositório (se não estiver no diretório)
#   7. Constrói as imagens Docker do YugabyteDB (:shared e :full)
#   8. Sobe o ambiente Shelter (docker compose up)
#   9. Verifica que todos os serviços estão saudáveis
#
# Uso:
#   chmod +x install.sh
#   ./install.sh
#
# Repositório: https://github.com/hub-unibloom/cascataV1.git
# =============================================================================

set -euo pipefail

# =============================================================================
# CORES E FORMATAÇÃO
# =============================================================================
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

REPO_URL="https://github.com/hub-unibloom/cascataV1.git"
GO_VERSION="1.26.0"
NODE_VERSION="20"
INSTALL_DIR="$(pwd)/cascataV1"

# =============================================================================
# FUNÇÕES AUXILIARES
# =============================================================================

log_header() {
    echo ""
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BOLD}${BLUE}  $1${NC}"
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
}

log_ok() {
    echo -e "  ${GREEN}✔${NC} $1"
}

log_warn() {
    echo -e "  ${YELLOW}⚠${NC} $1"
}

log_fail() {
    echo -e "  ${RED}✖${NC} $1"
}

log_info() {
    echo -e "  ${BLUE}→${NC} $1"
}

log_step() {
    echo -e "\n  ${BOLD}$1${NC}"
}

abort() {
    log_fail "$1"
    echo -e "\n${RED}Instalação abortada.${NC}"
    exit 1
}

command_exists() {
    command -v "$1" &>/dev/null
}

version_ge() {
    # Retorna 0 se $1 >= $2 (comparação semântica simples)
    printf '%s\n%s' "$2" "$1" | sort -V -C
}

# =============================================================================
# BANNER
# =============================================================================

echo ""
echo -e "${CYAN}"
echo "   ██████╗ █████╗ ███████╗ ██████╗ █████╗ ████████╗ █████╗ "
echo "  ██╔════╝██╔══██╗██╔════╝██╔════╝██╔══██╗╚══██╔══╝██╔══██╗"
echo "  ██║     ███████║███████╗██║     ███████║   ██║   ███████║"
echo "  ██║     ██╔══██║╚════██║██║     ██╔══██║   ██║   ██╔══██║"
echo "  ╚██████╗██║  ██║███████║╚██████╗██║  ██║   ██║   ██║  ██║"
echo "   ╚═════╝╚═╝  ╚═╝╚══════╝ ╚═════╝╚═╝  ╚═╝   ╚═╝   ╚═╝  ╚═╝"
echo -e "${NC}"
echo -e "${BOLD}  Instalador Automático — Cascata V1${NC}"
echo -e "  Orquestrador BaaS Multi-Tenant"
echo ""

# =============================================================================
# PASSO 0 — DETECTAR SISTEMA OPERACIONAL
# =============================================================================

log_header "PASSO 0 — Detectando sistema operacional"

OS="unknown"
DISTRO="unknown"
PKG_MANAGER="unknown"

if [[ -f /etc/os-release ]]; then
    source /etc/os-release
    OS="linux"
    DISTRO="${ID}"

    case "${ID}" in
        almalinux)
            PKG_MANAGER="dnf"
            ;;
        *)
            abort "Sistema incompatível. O Cascata V1 suporta ESTREITAMENTE o AlmaLinux 9 como Host OS."
            ;;
    esac

    VERSION_ID=$(echo "$VERSION_ID" | cut -d. -f1)
    if [[ "${VERSION_ID}" != "9" ]]; then
        abort "AlmaLinux detectado, mas a versão é ${VERSION_ID}. Requerido estritamente AlmaLinux 9."
    fi

    log_ok "Sistema: ${PRETTY_NAME}"
    log_ok "Distro: ${DISTRO} | Package Manager: ${PKG_MANAGER}"
else
    abort "Sistema operacional não suportado. Requerido AlmaLinux 9."
fi

ARCH=$(uname -m)
log_ok "Arquitetura: ${ARCH}"

# Mapear arch para nomes do Go
case "${ARCH}" in
    x86_64)  GO_ARCH="amd64" ;;
    aarch64) GO_ARCH="arm64" ;;
    armv7l)  GO_ARCH="armv6l" ;;
    *)       GO_ARCH="${ARCH}" ;;
esac

# =============================================================================
# PASSO 1 — VERIFICAR DOCKER (OBRIGATÓRIO)
# =============================================================================

log_header "PASSO 1 — Verificando Docker (obrigatório)"

if ! command_exists docker; then
    abort "Docker não encontrado. Execute os passos do pre-feito.txt antes de rodar este instalador."
fi

DOCKER_VERSION=$(docker --version | grep -oP '\d+\.\d+' | head -1)
log_ok "Docker Engine: v${DOCKER_VERSION}"

# Verificar Docker Compose plugin
if docker compose version &>/dev/null; then
    COMPOSE_VERSION=$(docker compose version --short 2>/dev/null || docker compose version | grep -oP '\d+\.\d+\.\d+' | head -1)
    log_ok "Docker Compose: v${COMPOSE_VERSION}"
else
    abort "Docker Compose plugin não encontrado. Instale com: sudo apt install docker-compose-plugin"
fi

# Verificar que o Docker daemon está rodando
if ! docker info &>/dev/null; then
    abort "Docker daemon não está rodando. Execute: sudo systemctl start docker"
fi
log_ok "Docker daemon: ativo"

# Verificar que o usuário pode usar Docker sem sudo
if ! docker ps &>/dev/null; then
    log_warn "Docker requer sudo. Adicionando usuário ao grupo docker..."
    sudo usermod -aG docker "$USER"
    log_warn "Execute 'newgrp docker' ou faça logout/login e rode este script novamente."
    exit 1
fi
log_ok "Docker: acessível sem sudo"

# =============================================================================
# PASSO 2 — VERIFICAR/INSTALAR GIT
# =============================================================================

log_header "PASSO 2 — Verificando Git"

if command_exists git; then
    GIT_VERSION=$(git --version | grep -oP '\d+\.\d+\.\d+')
    log_ok "Git: v${GIT_VERSION}"
else
    log_info "Git não encontrado. Instalando..."
    sudo dnf install -y -q git
    log_ok "Git instalado: $(git --version)"
fi

# =============================================================================
# PASSO 3 — VERIFICAR/INSTALAR GO 1.23+
# =============================================================================

log_header "PASSO 3 — Verificando Go ${GO_VERSION}+"

NEED_GO=false
if command_exists go; then
    CURRENT_GO=$(go version | grep -oP '\d+\.\d+\.\d+' | head -1)
    CURRENT_GO_MINOR=$(echo "${CURRENT_GO}" | cut -d. -f1,2)
    REQUIRED_GO_MINOR="1.26"

    if version_ge "${CURRENT_GO_MINOR}" "${REQUIRED_GO_MINOR}"; then
        log_ok "Go: v${CURRENT_GO} (>= ${REQUIRED_GO_MINOR} ✔)"
    else
        log_warn "Go v${CURRENT_GO} encontrado, mas precisa de >= ${REQUIRED_GO_MINOR}"
        NEED_GO=true
    fi
else
    log_info "Go não encontrado."
    NEED_GO=true
fi

if [[ "${NEED_GO}" == "true" ]]; then
    log_info "Instalando Go ${GO_VERSION}..."
    GO_TAR="go${GO_VERSION}.linux-${GO_ARCH}.tar.gz"
    wget -q "https://go.dev/dl/${GO_TAR}" -O "/tmp/${GO_TAR}"
    sudo rm -rf /usr/local/go
    sudo tar -C /usr/local -xzf "/tmp/${GO_TAR}"
    rm -f "/tmp/${GO_TAR}"

    # Adicionar ao PATH para esta sessão
    export PATH="/usr/local/go/bin:$PATH"
    export GOPATH="$HOME/go"
    export PATH="$GOPATH/bin:$PATH"

    # Persistir no .bashrc se ainda não está lá
    if ! grep -q '/usr/local/go/bin' "$HOME/.bashrc" 2>/dev/null; then
        {
            echo ''
            echo '# Go (adicionado pelo instalador Cascata)'
            echo 'export PATH=$PATH:/usr/local/go/bin'
            echo 'export GOPATH=$HOME/go'
            echo 'export PATH=$PATH:$GOPATH/bin'
        } >> "$HOME/.bashrc"
    fi

    log_ok "Go instalado: $(go version)"
fi

# =============================================================================
# PASSO 4 — VERIFICAR/INSTALAR RUST + CARGO
# =============================================================================

log_header "PASSO 4 — Verificando Rust + Cargo"

if command_exists rustc && command_exists cargo; then
    RUST_VERSION=$(rustc --version | grep -oP '\d+\.\d+\.\d+')
    log_ok "Rust: v${RUST_VERSION}"
    log_ok "Cargo: $(cargo --version | grep -oP '\d+\.\d+\.\d+')"
else
    log_info "Rust não encontrado. Instalando via rustup..."
    curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y --quiet
    source "$HOME/.cargo/env"
    log_ok "Rust instalado: $(rustc --version)"
    log_ok "Cargo instalado: $(cargo --version)"
fi

# =============================================================================
# PASSO 5 — VERIFICAR/INSTALAR NODE.JS 20+ VIA NVM
# =============================================================================

log_header "PASSO 5 — Verificando Node.js ${NODE_VERSION}+"

NEED_NODE=false
if command_exists node; then
    CURRENT_NODE=$(node --version | grep -oP '\d+' | head -1)
    if [[ "${CURRENT_NODE}" -ge "${NODE_VERSION}" ]]; then
        log_ok "Node.js: $(node --version)"
        log_ok "npm: $(npm --version)"
    else
        log_warn "Node.js v${CURRENT_NODE} encontrado, mas precisa de >= ${NODE_VERSION}"
        NEED_NODE=true
    fi
else
    log_info "Node.js não encontrado."
    NEED_NODE=true
fi

if [[ "${NEED_NODE}" == "true" ]]; then
    log_info "Instalando Node.js ${NODE_VERSION} via nvm..."

    # Instalar nvm se não existe
    if ! command_exists nvm && [[ ! -d "$HOME/.nvm" ]]; then
        curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.40.1/install.sh | bash
    fi

    # Carregar nvm
    export NVM_DIR="$HOME/.nvm"
    [ -s "$NVM_DIR/nvm.sh" ] && source "$NVM_DIR/nvm.sh"

    nvm install "${NODE_VERSION}" --silent
    nvm use "${NODE_VERSION}" --silent
    nvm alias default "${NODE_VERSION}"

    log_ok "Node.js instalado: $(node --version)"
    log_ok "npm instalado: $(npm --version)"
fi

# =============================================================================
# PASSO 6 — CLONAR REPOSITÓRIO E GARANTIR ESTRUTURA
# =============================================================================

log_header "PASSO 6 — Obtendo código-fonte"

# Detectar se já estamos dentro do repositório
if [[ -f "./TASK_Fase0_Alicerce_Invisivel.md" ]] || [[ -f "./SRS_CascataV1.md" ]]; then
    INSTALL_DIR="$(pwd)"
    log_ok "Já dentro do repositório: ${INSTALL_DIR}"
elif [[ -d "./cascataV1" ]] && [[ -f "./cascataV1/SRS_CascataV1.md" ]]; then
    INSTALL_DIR="$(pwd)/cascataV1"
    log_ok "Repositório encontrado: ${INSTALL_DIR}"
else
    log_info "Clonando repositório: ${REPO_URL}"
    git clone "${REPO_URL}" cascataV1
    INSTALL_DIR="$(pwd)/cascataV1"
    log_ok "Repositório clonado em: ${INSTALL_DIR}"
fi

cd "${INSTALL_DIR}"

# -------------------------------------------------------
# Garantir estrutura completa do monorepo (PR-1)
# Git não rastreia diretórios vazios — criamos aqui.
# -------------------------------------------------------
log_step "Garantindo estrutura do monorepo (PR-1)..."

DIRS_ALL=(
    "control-plane/cmd/cascata-cp"
    "control-plane/internal/config"
    "control-plane/internal/server"
    "control-plane/internal/tenant"
    "control-plane/internal/pool"
    "control-plane/internal/metadata"
    "control-plane/internal/recyclebin"
    "control-plane/internal/extensions"
    "control-plane/internal/scanning"
    "control-plane/internal/observability"
    "control-plane/internal/health"
    "control-plane/migrations"
    "gateway/src/middleware"
    "gateway/src/validation"
    "gateway/src/computed"
    "gateway/src/storage"
    "gateway/src/proxy"
    "gateway/src/error"
    "dashboard/src/routes"
    "dashboard/src/lib/components"
    "dashboard/src/lib/stores"
    "sdk/typescript/cascata-client"
    "sdk/typescript/cascata-compat"
    "sdk/cli/cascata-cli"
    "infra/docker/yugabytedb"

    "infra/docker/clickhouse"
    "infra/docker/vector"
    "infra/k8s/operators"
    "infra/cilium/policies"
    "scripts"
)

CREATED_COUNT=0
for dir in "${DIRS_ALL[@]}"; do
    if [[ ! -d "${dir}" ]]; then
        mkdir -p "${dir}"
        CREATED_COUNT=$((CREATED_COUNT + 1))
    fi
done

if [[ "${CREATED_COUNT}" -gt 0 ]]; then
    log_ok "${CREATED_COUNT} diretórios criados (git não rastreia dirs vazios)"
else
    log_ok "Todos os diretórios já existem"
fi

# Verificar componentes principais
for dir in "control-plane" "gateway" "dashboard" "sdk" "infra" "scripts"; do
    log_ok "${dir}/"
done

# =============================================================================
# PASSO 7 — CONSTRUIR IMAGENS DOCKER DO YUGABYTEDB
# =============================================================================

log_header "PASSO 7 — Construindo imagens Docker do YugabyteDB"

# Verificar se os Dockerfiles existem (PR-3 e PR-4)
if [[ ! -f "infra/docker/yugabytedb/Dockerfile.shared" ]] || \
   [[ $(wc -c < "infra/docker/yugabytedb/Dockerfile.shared") -lt 100 ]]; then
    log_warn "Dockerfile.shared ainda é placeholder (PR-3 pendente)."
    log_info "Usando imagem oficial do YugabyteDB por enquanto."
    SKIP_YB_BUILD=true
else
    SKIP_YB_BUILD=false
fi

if [[ "${SKIP_YB_BUILD}" == "false" ]]; then
    log_step "7a. cascata/yugabytedb:shared (Cat.1 + pg_cron)"
    log_info "Isto pode levar 5-10 minutos na primeira vez..."

    if docker images --format '{{.Repository}}:{{.Tag}}' | grep -q 'cascata/yugabytedb:shared'; then
        log_warn "Imagem cascata/yugabytedb:shared já existe. Pulando build."
        log_info "Para reconstruir: docker build -t cascata/yugabytedb:shared -f infra/docker/yugabytedb/Dockerfile.shared ."
    else
        docker build \
            -t cascata/yugabytedb:shared \
            -f infra/docker/yugabytedb/Dockerfile.shared \
            . 2>&1 | tail -5
        log_ok "cascata/yugabytedb:shared construída"
    fi

    echo ""

    if [[ -f "infra/docker/yugabytedb/Dockerfile.full" ]] && \
       [[ $(wc -c < "infra/docker/yugabytedb/Dockerfile.full") -gt 100 ]]; then
        log_step "7b. cascata/yugabytedb:full (Cat.1 + pg_cron + PostGIS)"
        log_info "Isto pode levar 15-25 minutos na primeira vez..."

        # Safety Net Memory Check for Makeflags
        TOTAL_RAM_MB=$(free -m | awk '/^Mem:/{print $2}')
        MAKEFLAGS_ARG="-j$(nproc)"
        if [[ "${TOTAL_RAM_MB}" -le 2500 ]]; then
            log_warn "Ram host detectada: ${TOTAL_RAM_MB}MB. Forçando MAKEFLAGS=-j2 para evitar OOM fatal do GCC."
            MAKEFLAGS_ARG="-j2"
        fi

        if docker images --format '{{.Repository}}:{{.Tag}}' | grep -q 'cascata/yugabytedb:full'; then
            log_warn "Imagem cascata/yugabytedb:full já existe. Pulando build."
            log_info "Para reconstruir: docker build --build-arg MAKEFLAGS=\"${MAKEFLAGS_ARG}\" -t cascata/yugabytedb:full -f infra/docker/yugabytedb/Dockerfile.full ."
        else
            docker build \
                --build-arg MAKEFLAGS="${MAKEFLAGS_ARG}" \
                -t cascata/yugabytedb:full \
                -f infra/docker/yugabytedb/Dockerfile.full \
                . 2>&1 | tail -5
            log_ok "cascata/yugabytedb:full construída"
        fi
    else
        log_warn "Dockerfile.full ainda é placeholder (PR-4 pendente). Pulando."
    fi

    echo ""
    log_step "Imagens Cascata disponíveis:"
    docker images --format "  {{.Repository}}:{{.Tag}}\t{{.Size}}\t{{.CreatedSince}}" | grep cascata || log_info "(nenhuma imagem cascata ainda)"
else
    log_info "Build de imagens pulado — será feito quando PR-3/PR-4 estiverem implementados."
fi

# =============================================================================
# PASSO 8 — SUBIR AMBIENTE SHELTER
# =============================================================================

log_header "PASSO 8 — Subindo ambiente Shelter (modo desenvolvimento)"

cd "${INSTALL_DIR}/infra/docker"

# Verificar se o docker-compose.shelter.yml existe e é real (não placeholder)
if [[ ! -f "docker-compose.shelter.yml" ]] || \
   [[ $(wc -l < "docker-compose.shelter.yml") -lt 20 ]]; then
    log_warn "docker-compose.shelter.yml ainda não está pronto (PR-2 pendente)."
    log_info "Pulando subida do ambiente. Finalize PR-2 e rode novamente."
else
    # Verificar se já está rodando
    RUNNING=$(docker compose -f docker-compose.shelter.yml ps --services --status running 2>/dev/null | wc -l)

    # -------------------------------------------------------

    if [[ "${RUNNING}" -gt 5 ]]; then
        log_warn "Ambiente Shelter já está rodando (${RUNNING} serviços). Pulando."
        log_info "Para reiniciar: docker compose -f docker-compose.shelter.yml down && docker compose -f docker-compose.shelter.yml up -d"
    else
        # Gateway e Control Plane precisam de Dockerfile — excluir se não existem
        COMPOSE_PROFILES=""
        EXCLUDE_SERVICES=""

        if [[ ! -f "${INSTALL_DIR}/gateway/Dockerfile" ]]; then
            log_warn "Gateway Dockerfile não encontrado (PR-6 pendente) — excluindo do startup"
            EXCLUDE_SERVICES="${EXCLUDE_SERVICES} --scale gateway=0"
        fi

        if [[ ! -f "${INSTALL_DIR}/control-plane/Dockerfile" ]]; then
            log_warn "Control Plane Dockerfile não encontrado (PR-5 pendente) — excluindo do startup"
            EXCLUDE_SERVICES="${EXCLUDE_SERVICES} --scale control-plane=0"
        fi

        log_info "Verificando e baixando dependências do Docker Hub (Pre-flight pull)..."
        if ! docker compose -f docker-compose.shelter.yml pull -q; then
            abort "Falha crítica: Uma ou mais imagens/tags definidos no docker-compose.shelter.yml não existem no repositório remoto."
        fi

        log_info "Iniciando serviços de infraestrutura..."
        eval docker compose -f docker-compose.shelter.yml up -d ${EXCLUDE_SERVICES} 2>&1

        log_info "Aguardando serviços ficarem saudáveis (até 120s)..."

        # Aguardar YugabyteDB (o mais lento)
        WAITED=0
        MAX_WAIT=120
        while [[ "${WAITED}" -lt "${MAX_WAIT}" ]]; do
            if docker exec cascata-yugabytedb bin/ysqlsh -c "SELECT 1" &>/dev/null; then
                break
            fi
            sleep 5
            WAITED=$((WAITED + 5))
            echo -ne "  ${BLUE}→${NC} Aguardando YugabyteDB... ${WAITED}s/${MAX_WAIT}s\r"
        done
        echo ""

        if [[ "${WAITED}" -ge "${MAX_WAIT}" ]]; then
            log_warn "YugabyteDB demorou mais que ${MAX_WAIT}s. Verifique: docker logs cascata-yugabytedb"
        else
            log_ok "YugabyteDB pronto em ${WAITED}s"
        fi
    fi
fi

# =============================================================================
# PASSO 9 — VERIFICAÇÃO FINAL
# =============================================================================

log_header "PASSO 9 — Verificação final de todos os serviços"

cd "${INSTALL_DIR}/infra/docker"

TOTAL_OK=0
TOTAL_FAIL=0

check_service() {
    local name=$1
    local check_cmd=$2
    local port=$3

    if eval "${check_cmd}" &>/dev/null; then
        log_ok "${name} (porta ${port})"
        TOTAL_OK=$((TOTAL_OK + 1))
    else
        log_fail "${name} (porta ${port}) — NÃO RESPONDEU"
        TOTAL_FAIL=$((TOTAL_FAIL + 1))
    fi
}

check_service "YugabyteDB"      "docker exec cascata-yugabytedb bin/ysqlsh -c 'SELECT 1'"            "5433"
check_service "DragonflyDB"     "docker exec cascata-dragonflydb redis-cli ping"                       "6379"
check_service "Redpanda"        "docker exec cascata-redpanda rpk cluster health"                      "9092"
check_service "ClickHouse"      "docker exec cascata-clickhouse clickhouse-client --user cascata --password 'cascata_dev_ch_2025' --query 'SELECT 1'"  "8123"
check_service "OpenBao"         "curl -sf http://localhost:8200/v1/sys/health"                          "8200"

check_service "VictoriaMetrics" "curl -sf http://localhost:8428/health"                                 "8428"
check_service "Vector"          "curl -sf http://localhost:8686/health"                                 "8686"

# Gateway e Control Plane precisam de build — podem não estar rodando ainda
if docker ps --format '{{.Names}}' | grep -q cascata-gateway; then
    check_service "Gateway" "curl -sf http://localhost:8080/health" "8080"
else
    log_warn "Gateway — ainda não construído (requer PR-6)"
fi

if docker ps --format '{{.Names}}' | grep -q cascata-control-plane; then
    check_service "Control Plane" "curl -sf http://localhost:9090/health" "9090"
else
    log_warn "Control Plane — ainda não construído (requer PR-5)"
fi

# =============================================================================
# PASSO 10 — RELATÓRIO DE MEMÓRIA
# =============================================================================

log_header "PASSO 10 — Uso de memória (modo Shelter)"

echo ""
docker stats --no-stream --format "  {{.Name}}\t{{.MemUsage}}\t{{.MemPerc}}" 2>/dev/null | sort || true
echo ""

TOTAL_MEM=$(docker stats --no-stream --format '{{.MemUsage}}' 2>/dev/null | grep -oP '[\d.]+MiB' | awk -F'MiB' '{sum+=$1} END {printf "%.0f", sum}')
if [[ -n "${TOTAL_MEM}" ]]; then
    log_info "Total RAM em uso: ~${TOTAL_MEM}MiB"
    if [[ "${TOTAL_MEM}" -lt 1536 ]]; then
        log_ok "Dentro do limite Shelter (< 1.5GB) ✔"
    else
        log_warn "Acima do limite Shelter esperado. Verifique os serviços."
    fi
fi

# =============================================================================
# RESUMO FINAL
# =============================================================================

log_header "INSTALAÇÃO CONCLUÍDA"

echo ""
echo -e "  ${GREEN}Serviços OK:   ${TOTAL_OK}${NC}"
if [[ "${TOTAL_FAIL}" -gt 0 ]]; then
    echo -e "  ${RED}Serviços FAIL: ${TOTAL_FAIL}${NC}"
fi
echo ""

echo -e "  ${BOLD}Portas principais:${NC}"
echo "    YugabyteDB YSQL:   localhost:5433"
echo "    YugabyteDB UI:     http://localhost:7100"
echo "    YSQL CM (pooled):  localhost:5433"
echo "    ClickHouse HTTP:   http://localhost:8123"
echo "    Redpanda Kafka:    localhost:9092"
echo "    DragonflyDB:       localhost:6379"
echo "    OpenBao:           http://localhost:8200"
echo "    VictoriaMetrics:   http://localhost:8428"
echo "    Gateway:           http://localhost:8080"
echo "    Control Plane:     http://localhost:9090"
echo ""

echo -e "  ${BOLD}Comandos úteis:${NC}"
echo "    Ver logs:     cd ${INSTALL_DIR}/infra/docker && docker compose -f docker-compose.shelter.yml logs -f"
echo "    Parar tudo:   cd ${INSTALL_DIR}/infra/docker && docker compose -f docker-compose.shelter.yml down"
echo "    Reiniciar:    cd ${INSTALL_DIR}/infra/docker && docker compose -f docker-compose.shelter.yml restart"
echo "    Conectar DB:  docker exec -it cascata-yugabytedb bin/ysqlsh"
echo ""

echo -e "  ${BOLD}Documentação:${NC}"
echo "    TASK file:    ${INSTALL_DIR}/TASK_Fase0_Alicerce_Invisivel.md"
echo "    SRS:          ${INSTALL_DIR}/SRS_CascataV1.md"
echo "    SAD:          ${INSTALL_DIR}/SAD_CascataV1.md"
echo ""

if [[ "${TOTAL_FAIL}" -eq 0 ]]; then
    echo -e "  ${GREEN}${BOLD}🚀 Cascata V1 pronto para desenvolvimento!${NC}"
else
    echo -e "  ${YELLOW}${BOLD}⚠  Alguns serviços não responderam. Verifique os logs.${NC}"
fi
echo ""
