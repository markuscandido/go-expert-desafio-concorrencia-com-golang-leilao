package auction_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/markuscandido/go-expert-desafio-concorrencia-com-golang-leilao/internal/entity/auction_entity"
	"github.com/stretchr/testify/assert"
)

func TestAuctionExpiresCorrectly(t *testing.T) {
	// Set auction interval to 1 second for testing
	os.Setenv("AUCTION_INTERVAL", "1s")
	defer os.Unsetenv("AUCTION_INTERVAL")

	// Create an auction
	auction, err := auction_entity.CreateAuction(
		"Test Product",
		"electronics",
		"This is a test product description for auction",
		auction_entity.New,
	)

	assert.Nil(t, err)
	assert.NotNil(t, auction)
	assert.Equal(t, auction_entity.Active, auction.Status)
	assert.False(t, auction.IsExpired())

	// ExpiresAt should be approximately 1 second after CreatedAt
	expectedDuration := 1 * time.Second
	actualDuration := auction.ExpiresAt.Sub(auction.CreatedAt)
	assert.InDelta(t, expectedDuration.Seconds(), actualDuration.Seconds(), 0.1)
}

func TestAuctionIsExpiredAfterInterval(t *testing.T) {
	// Set auction interval to 100 milliseconds for fast testing
	os.Setenv("AUCTION_INTERVAL", "100ms")
	defer os.Unsetenv("AUCTION_INTERVAL")

	// Create an auction
	auction, err := auction_entity.CreateAuction(
		"Test Product",
		"electronics",
		"This is a test product description for auction",
		auction_entity.New,
	)

	assert.Nil(t, err)
	assert.NotNil(t, auction)
	assert.False(t, auction.IsExpired(), "Auction should not be expired immediately after creation")

	// Wait for the auction to expire
	time.Sleep(150 * time.Millisecond)

	// Now the auction should be marked as expired
	assert.True(t, auction.IsExpired(), "Auction should be expired after the interval")
}

func TestAuctionCreatedAtAndExpiresAtAreSet(t *testing.T) {
	os.Setenv("AUCTION_INTERVAL", "5m")
	defer os.Unsetenv("AUCTION_INTERVAL")

	beforeCreation := time.Now()

	auction, err := auction_entity.CreateAuction(
		"Test Product",
		"test",
		"A description with more than 10 characters",
		auction_entity.Used,
	)

	afterCreation := time.Now()

	assert.Nil(t, err)
	assert.NotNil(t, auction)

	// CreatedAt should be between beforeCreation and afterCreation
	assert.True(t, auction.CreatedAt.After(beforeCreation) || auction.CreatedAt.Equal(beforeCreation))
	assert.True(t, auction.CreatedAt.Before(afterCreation) || auction.CreatedAt.Equal(afterCreation))

	// ExpiresAt should be 5 minutes after CreatedAt
	expectedExpiry := auction.CreatedAt.Add(5 * time.Minute)
	assert.Equal(t, expectedExpiry.Unix(), auction.ExpiresAt.Unix())
}

func TestCreateAuctionWithContext(t *testing.T) {
	os.Setenv("AUCTION_INTERVAL", "30s")
	defer os.Unsetenv("AUCTION_INTERVAL")

	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	auction, err := auction_entity.CreateAuction(
		"iPhone 15",
		"electronics",
		"Brand new iPhone 15 Pro Max 256GB",
		auction_entity.New,
	)

	assert.Nil(t, err)
	assert.Equal(t, "iPhone 15", auction.ProductName)
	assert.Equal(t, "electronics", auction.Category)
	assert.Equal(t, auction_entity.Active, auction.Status)
	assert.NotEmpty(t, auction.Id)

	// ExpiresAt should be 30 seconds after CreatedAt
	duration := auction.ExpiresAt.Sub(auction.CreatedAt)
	assert.Equal(t, 30*time.Second, duration)
}
