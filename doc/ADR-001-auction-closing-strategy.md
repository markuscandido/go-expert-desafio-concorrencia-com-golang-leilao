# ADR-001: Estratégia de Fechamento Automático de Leilões

**Data:** 2025-12-23  
**Status:** Aceito  
**Decisores:** Equipe de Desenvolvimento

---

## Contexto

O sistema de leilões precisa fechar automaticamente leilões que atingiram sua data de expiração (`expires_at`). Duas abordagens principais foram consideradas:

1. **Polling Global** - Uma única goroutine verificando periodicamente todos os leilões expirados
2. **Goroutine por Leilão** - Uma goroutine dedicada para cada leilão, disparando no momento exato de expiração

---

## Decisão

**Escolhemos a abordagem de Polling Global** com uma goroutine única que executa a cada `AUCTION_CLOSE_CHECK_INTERVAL` (padrão: 10 segundos), combinada com validação em tempo real no momento de criação de lances.

---

## Alternativas Consideradas

### Alternativa 1: Polling Global (Escolhida ✅)

```
┌─────────────────────────────────────────────────────────────────┐
│                    Goroutine Global                             │
│  ┌─────────┐                                                    │
│  │ Ticker  │──► A cada 10s: SELECT * FROM auctions              │
│  │  10s    │    WHERE status=0 AND expires_at <= now            │
│  └─────────┘    UPDATE SET status=1                             │
└─────────────────────────────────────────────────────────────────┘
```

**Implementação:** `internal/infra/database/auction/close_auction.go`

```go
func (ar *AuctionRepository) StartAuctionCloserRoutine(ctx context.Context) {
    ticker := time.NewTicker(interval)
    go func() {
        for {
            select {
            case <-ctx.Done():
                return
            case <-ticker.C:
                ar.closeExpiredAuctions(ctx)
            }
        }
    }()
}
```

### Alternativa 2: Goroutine por Leilão (Rejeitada ❌)

```
┌─────────────────────────────────────────────────────────────────┐
│  Leilão A (expira em 5m)     → goroutine com time.After(5m)     │
│  Leilão B (expira em 10m)    → goroutine com time.After(10m)    │
│  Leilão C (expira em 2m)     → goroutine com time.After(2m)     │
│  ...                                                            │
│  Leilão N (expira em Xm)     → goroutine com time.After(Xm)     │
└─────────────────────────────────────────────────────────────────┘
```

```go
// Exemplo de como seria implementado
func (ar *AuctionRepository) CreateAuction(ctx context.Context, auction *Auction) error {
    // ... salva no DB ...
    
    duration := time.Until(auction.ExpiresAt)
    time.AfterFunc(duration, func() {
        ar.CloseAuction(context.Background(), auction.Id)
    })
    
    return nil
}
```

---

## Análise Comparativa

| Aspecto | Polling Global | Goroutine por Leilão |
|---------|----------------|----------------------|
| **Precisão de fechamento** | ±10s (configurável) | Exata (milissegundos) |
| **Consumo de memória** | Constante (1 goroutine) | Linear O(N), ~2-4KB por leilão |
| **Sobrevive a restart** | ✅ Sim (reprocessa automaticamente) | ❌ Não (precisa recovery) |
| **Múltiplas instâncias** | ✅ Seguro (queries idempotentes) | ❌ Problemático (duplicação) |
| **Complexidade** | Baixa | Moderada/Alta |
| **Carga no DB** | Constante (1 query/intervalo) | Proporcional a criações |

---

## Justificativas da Decisão

### 1. A validação em tempo real elimina a necessidade de precisão

O sistema já implementa validação no momento de criação de lances:

```go
// internal/infra/database/bid/create_bid.go
if auctionStatus == auction_entity.Completed || now.After(auctionEndTime) {
    return // Lance rejeitado imediatamente
}
```

Isso significa que lances são **rejeitados instantaneamente** após `expires_at`, independente do status no banco. A goroutine de fechamento apenas atualiza o status para refletir a realidade — não é ela que impede lances.

### 2. Resiliência a restarts

Com polling global:
- Servidor reinicia → goroutine inicia → processa todos os expirados

Com goroutine por leilão:
- Servidor reinicia → todas as goroutines perdidas → precisa de lógica de recovery complexa

### 3. Escala horizontal

Em ambiente de produção com múltiplas instâncias:

**Polling Global:**
```
Instância 1: UpdateMany(expires_at <= now) → Atualiza 5 leilões
Instância 2: UpdateMany(expires_at <= now) → 0 leilões (já atualizados)
```
Operação idempotente, sem problemas.

**Goroutine por Leilão:**
```
Instância 1: CloseAuction(id=123)
Instância 2: CloseAuction(id=123)  // Duplicado!
```
Precisa de lock distribuído ou deduplicação.

### 4. Consumo de recursos

Cenário: 100.000 leilões ativos simultaneamente

| Métrica | Polling Global | Goroutine por Leilão |
|---------|----------------|----------------------|
| Goroutines | 1 | 100.000 |
| Memória (stack) | ~4KB | ~400MB |
| Queries/segundo | 0.1 (a cada 10s) | 0 (mas 100k timers) |

---

## Consequências

### Positivas

- ✅ Sistema simples e fácil de entender
- ✅ Baixo consumo de memória constante
- ✅ Resiliente a falhas e restarts
- ✅ Compatível com escala horizontal
- ✅ Fácil de monitorar (1 goroutine, logs centralizados)

### Negativas

- ⚠️ Atraso de até `AUCTION_CLOSE_CHECK_INTERVAL` para atualização do status
- ⚠️ Query periódica mesmo quando não há leilões para fechar

### Mitigações

1. **Atraso no status:** Mitigado pela validação em tempo real — lances já são rejeitados mesmo com status desatualizado
2. **Query desnecessária:** Impacto mínimo; query com índice em `status` e `expires_at` é O(log n)

---

## Quando Reconsiderar

Esta decisão deve ser reavaliada se:

1. **Precisão ao milissegundo** se tornar requisito crítico de negócio
2. **Volume de leilões** diminuir drasticamente (<100 ativos)
3. **Sistema single-instance** for garantido permanentemente
4. **Notificações em tempo real** precisarem do momento exato de fechamento

---

## Referências

- [Go Concurrency Patterns](https://go.dev/blog/pipelines)
- [time.Ticker vs time.AfterFunc](https://pkg.go.dev/time)
- Código fonte: `internal/infra/database/auction/close_auction.go`
- Código fonte: `internal/infra/database/bid/create_bid.go`
