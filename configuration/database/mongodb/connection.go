package mongodb

import (
	"context"
	"fmt"
	"os"

	"github.com/markuscandido/go-expert-desafio-concorrencia-com-golang-leilao/configuration/logger"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Constantes para variáveis de ambiente do MongoDB
const (
	MONGODB_URL      = "MONGODB_URL"      // URL completa (fallback para compatibilidade)
	MONGODB_HOST     = "MONGODB_HOST"     // Host do MongoDB (padrão: localhost)
	MONGODB_PORT     = "MONGODB_PORT"     // Porta do MongoDB (padrão: 27017)
	MONGODB_USER     = "MONGODB_USER"     // Usuário (opcional)
	MONGODB_PASSWORD = "MONGODB_PASSWORD" // Senha (opcional)
	MONGODB_DB       = "MONGODB_DB"       // Nome do banco de dados
)

// buildMongoURI constrói a URI de conexão do MongoDB a partir das variáveis de ambiente.
// Prioridade: MONGODB_URL (se definida) > construção a partir de componentes
func buildMongoURI() string {
	// Se MONGODB_URL está definida, usa ela diretamente (compatibilidade retroativa)
	if mongoURL := os.Getenv(MONGODB_URL); mongoURL != "" {
		return mongoURL
	}

	host := getEnvOrDefault(MONGODB_HOST, "localhost")
	port := getEnvOrDefault(MONGODB_PORT, "27017")
	user := os.Getenv(MONGODB_USER)
	password := os.Getenv(MONGODB_PASSWORD)
	database := os.Getenv(MONGODB_DB)

	// Se user e password estão definidos, usa autenticação
	if user != "" && password != "" {
		return fmt.Sprintf("mongodb://%s:%s@%s:%s/%s?authSource=admin",
			user, password, host, port, database)
	}

	// Conexão sem autenticação
	return fmt.Sprintf("mongodb://%s:%s", host, port)
}

// getEnvOrDefault retorna o valor da variável de ambiente ou um valor padrão
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// NewMongoDBConnection estabelece uma conexão com o MongoDB
func NewMongoDBConnection(ctx context.Context) (*mongo.Database, error) {
	mongoURI := buildMongoURI()
	mongoDatabase := os.Getenv(MONGODB_DB)

	logger.Info(fmt.Sprintf("Connecting to MongoDB at: %s", maskPassword(mongoURI)))

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		logger.Error("Error trying to connect to mongodb database", err)
		return nil, err
	}

	if err := client.Ping(ctx, nil); err != nil {
		logger.Error("Error trying to ping mongodb database", err)
		return nil, err
	}

	logger.Info(fmt.Sprintf("Successfully connected to MongoDB database: %s", mongoDatabase))
	return client.Database(mongoDatabase), nil
}

// maskPassword oculta a senha na URI para exibição em logs
func maskPassword(uri string) string {
	// Simples masking: substitui a senha por asteriscos
	// Pattern: mongodb://user:password@host -> mongodb://user:****@host
	for i := 0; i < len(uri); i++ {
		if i+2 < len(uri) && uri[i:i+3] == "://" {
			start := i + 3
			for j := start; j < len(uri); j++ {
				if uri[j] == ':' {
					// Encontrou o separador user:password
					passStart := j + 1
					for k := passStart; k < len(uri); k++ {
						if uri[k] == '@' {
							// Encontrou o fim da senha
							return uri[:passStart] + "****" + uri[k:]
						}
					}
				}
				if uri[j] == '@' {
					break
				}
			}
		}
	}
	return uri
}
