package auction_entity

import (
	"context"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/markuscandido/go-expert-desafio-concorrencia-com-golang-leilao/internal/internal_error"
)

func CreateAuction(
	productName, category, description string,
	condition ProductCondition) (*Auction, *internal_error.InternalError) {

	now := time.Now()
	expiresAt := now.Add(getAuctionInterval())

	auction := &Auction{
		Id:          uuid.New().String(),
		ProductName: productName,
		Category:    category,
		Description: description,
		Condition:   condition,
		Status:      Active,
		CreatedAt:   now,
		ExpiresAt:   expiresAt,
	}

	if err := auction.Validate(); err != nil {
		return nil, err
	}

	return auction, nil
}

func (au *Auction) Validate() *internal_error.InternalError {
	if len(au.ProductName) <= 1 ||
		len(au.Category) <= 2 ||
		len(au.Description) <= 10 && (au.Condition != New &&
			au.Condition != Refurbished &&
			au.Condition != Used) {
		return internal_error.NewBadRequestError("invalid auction object")
	}

	return nil
}

// IsExpired checks if the auction has expired
func (au *Auction) IsExpired() bool {
	return time.Now().After(au.ExpiresAt)
}

type Auction struct {
	Id          string
	ProductName string
	Category    string
	Description string
	Condition   ProductCondition
	Status      AuctionStatus
	CreatedAt   time.Time // Data de criação
	ExpiresAt   time.Time // Data de expiração (calculada automaticamente)
}

type ProductCondition int
type AuctionStatus int

const (
	Active AuctionStatus = iota
	Completed
)

const (
	New ProductCondition = iota + 1
	Used
	Refurbished
)

type AuctionRepositoryInterface interface {
	CreateAuction(
		ctx context.Context,
		auctionEntity *Auction) *internal_error.InternalError

	FindAuctions(
		ctx context.Context,
		status AuctionStatus,
		category, productName string) ([]Auction, *internal_error.InternalError)

	FindAuctionById(
		ctx context.Context, id string) (*Auction, *internal_error.InternalError)
}

// getAuctionInterval returns the auction duration from env var
func getAuctionInterval() time.Duration {
	auctionInterval := os.Getenv("AUCTION_INTERVAL")
	duration, err := time.ParseDuration(auctionInterval)
	if err != nil {
		return 5 * time.Minute // Default: 5 minutes
	}
	return duration
}
