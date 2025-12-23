# =============================================================================
# Multi-stage Dockerfile para Go Auction System
# =============================================================================
# Estágio 1: Builder - Compila a aplicação
# Estágio 2: Compressor - Comprime o binário com UPX
# Estágio 3: Runner - Imagem scratch mínima
# =============================================================================

# -----------------------------------------------------------------------------
# ESTÁGIO 1: Builder
# -----------------------------------------------------------------------------
FROM golang:1.25-alpine AS builder

# Instala certificados CA para requisições HTTPS
RUN apk add --no-cache ca-certificates

WORKDIR /app

# Copia arquivos de dependências primeiro (para cache de layers)
COPY go.mod go.sum ./
RUN go mod download

# Copia o código fonte
COPY . .

# Compila o binário estático
# CGO_ENABLED=0: Desabilita CGO para binário 100% estático
# -trimpath: Remove paths absolutos do binário (segurança + menor tamanho)
# -ldflags="-w -s": Remove símbolos de debug (reduz tamanho)
# -o: Nome do binário de saída
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -trimpath \
    -ldflags="-w -s" \
    -o /app/auction \
    cmd/auction/main.go

# -----------------------------------------------------------------------------
# ESTÁGIO 2: Compressor (UPX)
# -----------------------------------------------------------------------------
FROM alpine:3.19 AS compressor

# Instala UPX
RUN apk add --no-cache upx

# Copia o binário do estágio anterior
COPY --from=builder /app/auction /auction

# Comprime o binário (--best = máxima compressão, --lzma = melhor algoritmo)
RUN upx --best --lzma /auction

# -----------------------------------------------------------------------------
# ESTÁGIO 3: Runner (Scratch)
# -----------------------------------------------------------------------------
FROM scratch

# Copia certificados CA do estágio builder (necessário para HTTPS)
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copia o binário COMPRIMIDO do estágio compressor
COPY --from=compressor /auction /auction

# Copia o arquivo de configuração .env
COPY cmd/auction/.env /cmd/auction/.env

# Expõe a porta da aplicação
EXPOSE 8080

# Executa o binário
ENTRYPOINT ["/auction"]