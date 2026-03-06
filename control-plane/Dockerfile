# =============================================================================
# cascata/control-plane — Dockerfile
# =============================================================================
# Multi-stage: builder (Go compile) → runtime (Alpine minimal).
# Zero ferramentas de build no runtime. Binário estático.
# Ref: TASK PR-5, SAD §A.
# =============================================================================

# Stage 1: Builder
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /build

# Cache de dependências — go.mod/go.sum mudam menos que o código
COPY go.mod ./
COPY go.sum* ./
RUN go mod download 2>/dev/null || true

# Copiar código e compilar
COPY . .
RUN go mod tidy 2>/dev/null || true
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w -X github.com/cascata-platform/cascata/control-plane/internal/server.Version=$(cat VERSION 2>/dev/null || echo 0.1.0)" \
    -o cascata-cp \
    ./cmd/cascata-cp/

# Stage 2: Runtime — imagem mínima
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

# Criar user não-root
RUN adduser -D -h /app cascata
USER cascata
WORKDIR /app

# Copiar binário do builder
COPY --from=builder /build/cascata-cp .

# Copiar config de dev (sobrescrita por mount em produção)
COPY --from=builder /build/config.dev.yaml /etc/cascata/config.yaml

EXPOSE 9090

ENV CASCATA_CONFIG_PATH=/etc/cascata/config.yaml

ENTRYPOINT ["./cascata-cp"]
