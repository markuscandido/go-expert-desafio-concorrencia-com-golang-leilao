# Arquitetura do Sistema de Leilão

## Visão Geral

O sistema segue os princípios da **Clean Architecture**, separando responsabilidades em camadas bem definidas que permitem alta testabilidade, manutenibilidade e independência de frameworks externos.

## Diagrama de Camadas

```mermaid
graph TB
    subgraph "Camada de Apresentação"
        A[HTTP Controllers - Gin]
        B[Validation]
    end

    subgraph "Camada de Aplicação"
        C[Use Cases]
        D[DTOs]
    end

    subgraph "Camada de Domínio"
        E[Entities]
        F[Repository Interfaces]
    end

    subgraph "Camada de Infraestrutura"
        G[MongoDB Repositories]
        H[Logger]
        I[Database Connection]
        J[Auction Close Routine]
    end

    A --> C
    B --> A
    C --> E
    C --> F
    G --> F
    G --> I
    C --> D
    J --> G
```

## Estrutura de Diretórios

```
├── .env                         # Variáveis de ambiente (raiz do projeto)
├── cmd/
│   └── auction/
│       └── main.go              # Ponto de entrada, injeção de dependências
│
├── configuration/
│   ├── database/mongodb/        # Conexão com MongoDB
│   ├── logger/                  # Logger estruturado (Zap)
│   └── rest_err/                # Padronização de erros REST
│
├── internal/
│   ├── entity/                  # Entidades de Domínio
│   │   ├── auction_entity/      # Leilão
│   │   ├── bid_entity/          # Lance
│   │   └── user_entity/         # Usuário
│   │
│   ├── infra/
│   │   ├── api/web/
│   │   │   ├── controller/      # Controladores HTTP
│   │   │   └── validation/      # Validação de requests
│   │   │
│   │   └── database/            # Implementação dos repositórios
│   │       ├── auction/
│   │       │   ├── create_auction.go
│   │       │   ├── find_auction.go
│   │       │   └── close_auction.go  # Goroutine de fechamento
│   │       ├── bid/
│   │       └── user/
│   │
│   ├── internal_error/          # Tipos de erro internos
│   │
│   └── usecase/                 # Casos de uso
│       ├── auction_usecase/
│       ├── bid_usecase/
│       └── user_usecase/
│
└── doc/                         # Documentação
```

## Camadas da Arquitetura

### 1. Camada de Domínio (Entities)

A camada mais interna contém as **entidades de negócio** e suas regras:

| Entidade | Arquivo | Responsabilidade |
|----------|---------|------------------|
| `Auction` | `auction_entity.go` | Leilão com validação, estados, campos `CreatedAt` e `ExpiresAt` |
| `Bid` | `bid_entity.go` | Lance com validação e interface do repositório |
| `User` | `user_entity.go` | Usuário e interface do repositório |

**Características:**
- Entidades são independentes de frameworks
- Contêm regras de validação de negócio
- Definem interfaces que devem ser implementadas pela infraestrutura

### 2. Camada de Aplicação (Use Cases)

Orquestra o fluxo de dados entre entidades e infraestrutura:

| Use Case | Responsabilidade |
|----------|------------------|
| `AuctionUseCase` | Criar, buscar leilões e determinar vencedor |
| `BidUseCase` | Criar e buscar lances |
| `UserUseCase` | Buscar usuários |

**DTOs (Data Transfer Objects):**
- `AuctionInputDTO` / `AuctionOutputDTO`
- `BidInputDTO` / `BidOutputDTO`
- `WinningInfoOutputDTO`

### 3. Camada de Infraestrutura

Implementações concretas das interfaces:

| Componente | Tecnologia | Responsabilidade |
|------------|------------|------------------|
| Repositories | MongoDB | Persistência de dados |
| Controllers | Gin | Roteamento e serialização HTTP |
| Logger | Zap | Logs estruturados |
| Close Routine | Goroutine | Fechamento automático de leilões |

### 4. Camada de Apresentação (Controllers)

Recebe requisições HTTP e delega para os use cases:

- `AuctionController` - Endpoints de leilão
- `BidController` - Endpoints de lance
- `UserController` - Endpoints de usuário

## Fluxo de Dependências

```mermaid
graph LR
    A[main.go] --> B[Controllers]
    A --> C[Use Cases]
    A --> D[Repositories]
    A --> E[Close Routine]
    
    B --> C
    C --> F[Entity Interfaces]
    D --> F
    E --> D
    
    style F fill:#f9f,stroke:#333,stroke-width:2px
```

A regra de dependência diz que camadas externas podem depender de camadas internas, mas nunca o contrário. As **interfaces definidas nas entidades** permitem que os use cases não dependam diretamente das implementações de infraestrutura.

## Inversão de Dependência

O padrão é aplicado através de:

1. **Interfaces nas Entidades:**
   ```go
   type AuctionRepositoryInterface interface {
       CreateAuction(ctx context.Context, auction *Auction) *internal_error.InternalError
       FindAuctions(...) ([]Auction, *internal_error.InternalError)
       FindAuctionById(ctx context.Context, id string) (*Auction, *internal_error.InternalError)
   }
   ```

2. **Injeção de Dependências no main.go:**
   ```go
   auctionRepository := auction.NewAuctionRepository(database)
   auctionController := auction_controller.NewAuctionController(
       auction_usecase.NewAuctionUseCase(auctionRepository, bidRepository))
   
   // Inicia goroutine de fechamento automático
   auctionRepository.StartAuctionCloserRoutine(ctx)
   ```

## Tratamento de Erros

O sistema utiliza um padrão centralizado de erros:

| Tipo | HTTP Status | Uso |
|------|-------------|-----|
| `BadRequestError` | 400 | Validação de entrada |
| `NotFoundError` | 404 | Recurso não encontrado |
| `InternalServerError` | 500 | Erros internos |

## Concorrência

O sistema implementa **três mecanismos de concorrência**:

### 1. Processamento de Lances (`BidRepository`)

- `sync.WaitGroup` para sincronização de goroutines
- `sync.Mutex` para proteção de mapas compartilhados
- Cache em memória para status e tempo de expiração de leilões

### 2. Goroutine de Fechamento Automático (`close_auction.go`)

- Executa em background a cada `AUCTION_CLOSE_CHECK_INTERVAL`
- Usa `time.Ticker` para execução periódica
- Respeita `context.Done()` para shutdown graceful

### 3. Validação em Tempo Real de Expiração

- Lances são rejeitados **imediatamente** se `now > expires_at`
- Não depende da goroutine de fechamento ter atualizado o status
- Garante zero lances após expiração

## Configuração

O arquivo `.env` deve estar na **raiz do projeto**. A aplicação carrega automaticamente as variáveis de ambiente no startup.

Veja [BUSINESS_RULES.md](BUSINESS_RULES.md) para detalhes das variáveis de configuração.

Veja [DATA_FLOW.md](DATA_FLOW.md) para detalhes do fluxo de criação de lances.

## Architecture Decision Records (ADRs)

- [ADR-001: Estratégia de Fechamento Automático de Leilões](ADR-001-auction-closing-strategy.md)
