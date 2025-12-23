package auction

import (
	"context"
	"os"
	"time"

	"github.com/markuscandido/go-expert-desafio-concorrencia-com-golang-leilao/configuration/logger"
	"github.com/markuscandido/go-expert-desafio-concorrencia-com-golang-leilao/internal/entity/auction_entity"
	"go.mongodb.org/mongo-driver/bson"
)

// StartAuctionCloserRoutine starts a background goroutine that periodically
// checks for expired auctions and closes them automatically.
func (ar *AuctionRepository) StartAuctionCloserRoutine(ctx context.Context) {
	interval := getCloseCheckInterval()
	ticker := time.NewTicker(interval)

	logger.Info("Starting auction closer routine, checking every " + interval.String())

	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				logger.Info("Auction closer routine stopped")
				return
			case <-ticker.C:
				ar.closeExpiredAuctions(ctx)
			}
		}
	}()
}

// closeExpiredAuctions finds all active auctions that have expired and marks them as completed.
func (ar *AuctionRepository) closeExpiredAuctions(ctx context.Context) {
	now := time.Now().Unix()

	filter := bson.M{
		"status":     auction_entity.Active,
		"expires_at": bson.M{"$lte": now},
	}

	update := bson.M{
		"$set": bson.M{"status": auction_entity.Completed},
	}

	result, err := ar.Collection.UpdateMany(ctx, filter, update)
	if err != nil {
		logger.Error("Error closing expired auctions", err)
		return
	}

	if result.ModifiedCount > 0 {
		logger.Info("Closed " + string(rune(result.ModifiedCount)) + " expired auction(s)")
	}
}

// getCloseCheckInterval returns the interval for checking expired auctions.
// Default: 10 seconds. Configurable via AUCTION_CLOSE_CHECK_INTERVAL env var.
func getCloseCheckInterval() time.Duration {
	interval := os.Getenv("AUCTION_CLOSE_CHECK_INTERVAL")
	duration, err := time.ParseDuration(interval)
	if err != nil {
		return 10 * time.Second // Default: 10 seconds
	}
	return duration
}
