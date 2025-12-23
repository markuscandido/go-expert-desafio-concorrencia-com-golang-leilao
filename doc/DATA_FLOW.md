# Fluxo de Dados

Este documento descreve os principais fluxos de dados do sistema de leilão.

## Criar Leilão

```mermaid
sequenceDiagram
    participant Client
    participant Controller
    participant UseCase
    participant Entity
    participant Repository
    participant MongoDB

    Client->>Controller: POST /auction (JSON)
    Controller->>Controller: Validar JSON (binding)
    Controller->>UseCase: CreateAuction(AuctionInputDTO)
    UseCase->>Entity: CreateAuction(params)
    Entity->>Entity: Validate()
    Entity-->>UseCase: *Auction
    UseCase->>Repository: CreateAuction(ctx, auction)
    Repository->>MongoDB: InsertOne()
    MongoDB-->>Repository: OK
    Repository-->>UseCase: nil
    UseCase-->>Controller: nil
    Controller-->>Client: 201 Created
```

### Validações

1. **Controller (binding):**
   - `product_name`: obrigatório, mínimo 1 caractere
   - `category`: obrigatório, mínimo 2 caracteres
   - `description`: obrigatório, 10-200 caracteres
   - `condition`: deve ser 0, 1 ou 2

2. **Entity:**
   - Valida regras de negócio adicionais
   - Gera UUID e timestamp automaticamente

---

## Criar Lance (com Concorrência)

O sistema de lances implementa processamento concorrente com controle de expiração:

```mermaid
sequenceDiagram
    participant Client
    participant Controller
    participant UseCase
    participant Repository
    participant Cache
    participant MongoDB

    Client->>Controller: POST /bid (JSON)
    Controller->>UseCase: CreateBid(BidInputDTO)
    UseCase->>Repository: CreateBid(ctx, []Bid)
    
    loop Para cada lance (goroutine)
        Repository->>Cache: Verificar status do leilão
        alt Cache encontrado
            Repository->>Repository: Verificar se leilão expirou
            alt Leilão ativo
                Repository->>MongoDB: InsertOne()
            else Leilão expirado
                Repository->>Repository: Ignorar lance
            end
        else Cache não encontrado
            Repository->>MongoDB: FindAuctionById()
            MongoDB-->>Repository: Auction
            Repository->>Cache: Armazenar status e fim
            Repository->>MongoDB: InsertOne()
        end
    end
    
    Repository-->>UseCase: nil
    UseCase-->>Controller: nil
    Controller-->>Client: 201 Created
```

### Controle de Concorrência

O `BidRepository` mantém dois mapas protegidos por mutex:

| Mapa | Proteção | Finalidade |
|------|----------|------------|
| `auctionStatusMap` | `auctionStatusMapMutex` | Cache do status do leilão |
| `auctionEndTimeMap` | `auctionEndTimeMutex` | Cache do tempo de expiração |

### Tempo de Expiração

```go
auctionEndTime = auction.Timestamp.Add(auctionInterval)
```

A duração do leilão (`AUCTION_INTERVAL`) é configurável via variável de ambiente, com padrão de 5 minutos.

---

## Buscar Lance Vencedor

```mermaid
sequenceDiagram
    participant Client
    participant Controller
    participant UseCase
    participant AuctionRepo
    participant BidRepo
    participant MongoDB

    Client->>Controller: GET /auction/winner/:auctionId
    Controller->>Controller: Validar UUID
    Controller->>UseCase: FindWinningBidByAuctionId(ctx, id)
    UseCase->>AuctionRepo: FindAuctionById(ctx, id)
    AuctionRepo->>MongoDB: FindOne()
    MongoDB-->>AuctionRepo: Auction
    UseCase->>BidRepo: FindWinningBidByAuctionId(ctx, id)
    BidRepo->>MongoDB: FindOne() ordenado por amount DESC
    MongoDB-->>BidRepo: Bid (maior lance)
    BidRepo-->>UseCase: *Bid
    UseCase-->>Controller: WinningInfoOutputDTO
    Controller-->>Client: 200 OK (JSON)
```

---

## Listar Leilões com Filtros

```mermaid
sequenceDiagram
    participant Client
    participant Controller
    participant UseCase
    participant Repository
    participant MongoDB

    Client->>Controller: GET /auction?status=0&category=electronics
    Controller->>Controller: Converter status para int
    Controller->>UseCase: FindAuctions(ctx, status, category, productName)
    UseCase->>Repository: FindAuctions(ctx, status, category, productName)
    Repository->>MongoDB: Find() com filtros
    MongoDB-->>Repository: []AuctionEntityMongo
    Repository->>Repository: Converter para []Auction
    Repository-->>UseCase: []Auction
    UseCase->>UseCase: Converter para []AuctionOutputDTO
    UseCase-->>Controller: []AuctionOutputDTO
    Controller-->>Client: 200 OK (JSON Array)
```

### Filtros Disponíveis

| Query Param | Tipo | Descrição |
|-------------|------|-----------|
| `status` | int | 0 = Ativo, 1 = Completado |
| `category` | string | Filtro por categoria |
| `productName` | string | Filtro por nome do produto |

---

## Transformação de Dados

### Entity → MongoDB Document

```go
// Auction Entity
type Auction struct {
    Id          string
    ProductName string
    Timestamp   time.Time  // Go time
}

// MongoDB Document
type AuctionEntityMongo struct {
    Id          string `bson:"_id"`
    ProductName string `bson:"product_name"`
    Timestamp   int64  `bson:"timestamp"`  // Unix timestamp
}
```

### Entity → DTO (Output)

```go
// Auction Entity
type Auction struct {
    Id          string
    Condition   ProductCondition  // entity type
    Status      AuctionStatus     // entity type
}

// Output DTO
type AuctionOutputDTO struct {
    Id          string           `json:"id"`
    Condition   ProductCondition `json:"condition"`   // usecase type (int64)
    Status      AuctionStatus    `json:"status"`      // usecase type (int64)
}
```

As conversões de tipo entre camadas garantem o desacoplamento e permitem diferentes representações para cada contexto.
