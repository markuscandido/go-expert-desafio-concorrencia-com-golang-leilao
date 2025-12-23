package auction

import (
	"context"

	"github.com/markuscandido/go-expert-desafio-concorrencia-com-golang-leilao/configuration/logger"
	"github.com/markuscandido/go-expert-desafio-concorrencia-com-golang-leilao/internal/entity/auction_entity"
	"github.com/markuscandido/go-expert-desafio-concorrencia-com-golang-leilao/internal/internal_error"

	"go.mongodb.org/mongo-driver/mongo"
)

type AuctionEntityMongo struct {
	Id          string                          `bson:"_id"`
	ProductName string                          `bson:"product_name"`
	Category    string                          `bson:"category"`
	Description string                          `bson:"description"`
	Condition   auction_entity.ProductCondition `bson:"condition"`
	Status      auction_entity.AuctionStatus    `bson:"status"`
	CreatedAt   int64                           `bson:"created_at"`
	ExpiresAt   int64                           `bson:"expires_at"`
}

type AuctionRepository struct {
	Collection *mongo.Collection
}

func NewAuctionRepository(database *mongo.Database) *AuctionRepository {
	return &AuctionRepository{
		Collection: database.Collection("auctions"),
	}
}

func (ar *AuctionRepository) CreateAuction(
	ctx context.Context,
	auctionEntity *auction_entity.Auction) *internal_error.InternalError {
	auctionEntityMongo := &AuctionEntityMongo{
		Id:          auctionEntity.Id,
		ProductName: auctionEntity.ProductName,
		Category:    auctionEntity.Category,
		Description: auctionEntity.Description,
		Condition:   auctionEntity.Condition,
		Status:      auctionEntity.Status,
		CreatedAt:   auctionEntity.CreatedAt.Unix(),
		ExpiresAt:   auctionEntity.ExpiresAt.Unix(),
	}
	_, err := ar.Collection.InsertOne(ctx, auctionEntityMongo)
	if err != nil {
		logger.Error("Error trying to insert auction", err)
		return internal_error.NewInternalServerError("Error trying to insert auction")
	}

	return nil
}
