package bid_usecase

import (
	"context"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/markuscandido/go-expert-desafio-concorrencia-com-golang-leilao/configuration/logger"
	"github.com/markuscandido/go-expert-desafio-concorrencia-com-golang-leilao/internal/entity/auction_entity"
	"github.com/markuscandido/go-expert-desafio-concorrencia-com-golang-leilao/internal/entity/bid_entity"
	"github.com/markuscandido/go-expert-desafio-concorrencia-com-golang-leilao/internal/entity/user_entity"
	"github.com/markuscandido/go-expert-desafio-concorrencia-com-golang-leilao/internal/internal_error"
)

type BidInputDTO struct {
	UserId    string  `json:"user_id"`
	AuctionId string  `json:"auction_id"`
	Amount    float64 `json:"amount"`
}

type BidOutputDTO struct {
	Id        string    `json:"id"`
	UserId    string    `json:"user_id"`
	AuctionId string    `json:"auction_id"`
	Amount    float64   `json:"amount"`
	Timestamp time.Time `json:"timestamp" time_format:"2006-01-02 15:04:05"`
}

type BidUseCase struct {
	BidRepository     bid_entity.BidEntityRepository
	AuctionRepository auction_entity.AuctionRepositoryInterface
	UserRepository    user_entity.UserRepositoryInterface

	timer               *time.Timer
	maxBatchSize        int
	batchInsertInterval time.Duration
	bidChannel          chan bid_entity.Bid
	bidBatch            []bid_entity.Bid
	bidBatchMutex       *sync.Mutex

	// Pending bids cache - tracks highest bid per auction before persistence
	pendingHighestBid      map[string]*bid_entity.Bid // auctionId -> highest pending bid
	pendingHighestBidMutex *sync.RWMutex
}

func NewBidUseCase(
	bidRepository bid_entity.BidEntityRepository,
	auctionRepository auction_entity.AuctionRepositoryInterface,
	userRepository user_entity.UserRepositoryInterface,
) BidUseCaseInterface {
	maxSizeInterval := getMaxBatchSizeInterval()
	maxBatchSize := getMaxBatchSize()

	bidUseCase := &BidUseCase{
		BidRepository:          bidRepository,
		AuctionRepository:      auctionRepository,
		UserRepository:         userRepository,
		maxBatchSize:           maxBatchSize,
		batchInsertInterval:    maxSizeInterval,
		timer:                  time.NewTimer(maxSizeInterval),
		bidChannel:             make(chan bid_entity.Bid, maxBatchSize),
		bidBatch:               make([]bid_entity.Bid, 0),
		bidBatchMutex:          &sync.Mutex{},
		pendingHighestBid:      make(map[string]*bid_entity.Bid),
		pendingHighestBidMutex: &sync.RWMutex{},
	}

	bidUseCase.triggerCreateRoutine(context.Background())

	return bidUseCase
}

type BidUseCaseInterface interface {
	CreateBid(
		ctx context.Context,
		bidInputDTO BidInputDTO) *internal_error.InternalError

	FindWinningBidByAuctionId(
		ctx context.Context, auctionId string) (*BidOutputDTO, *internal_error.InternalError)

	FindBidByAuctionId(
		ctx context.Context, auctionId string) ([]BidOutputDTO, *internal_error.InternalError)
}

func (bu *BidUseCase) triggerCreateRoutine(ctx context.Context) {
	go func() {
		defer close(bu.bidChannel)

		for {
			select {
			case bidEntity, ok := <-bu.bidChannel:
				if !ok {
					bu.bidBatchMutex.Lock()
					if len(bu.bidBatch) > 0 {
						if err := bu.BidRepository.CreateBid(ctx, bu.bidBatch); err != nil {
							logger.Error("error trying to process bid batch list", err)
						}
					}
					bu.bidBatchMutex.Unlock()
					return
				}

				bu.bidBatchMutex.Lock()
				bu.bidBatch = append(bu.bidBatch, bidEntity)

				if len(bu.bidBatch) >= bu.maxBatchSize {
					if err := bu.BidRepository.CreateBid(ctx, bu.bidBatch); err != nil {
						logger.Error("error trying to process bid batch list", err)
					}

					bu.bidBatch = nil
					bu.timer.Reset(bu.batchInsertInterval)
				}
				bu.bidBatchMutex.Unlock()

			case <-bu.timer.C:
				bu.bidBatchMutex.Lock()
				if len(bu.bidBatch) > 0 {
					if err := bu.BidRepository.CreateBid(ctx, bu.bidBatch); err != nil {
						logger.Error("error trying to process bid batch list", err)
					}
				}
				bu.bidBatch = nil
				bu.timer.Reset(bu.batchInsertInterval)
				bu.bidBatchMutex.Unlock()
			}
		}
	}()
}

// clearPendingBidsCache clears all pending bids after they are persisted
func (bu *BidUseCase) clearPendingBidsCache() {
	bu.pendingHighestBidMutex.Lock()
	bu.pendingHighestBid = make(map[string]*bid_entity.Bid)
	bu.pendingHighestBidMutex.Unlock()
}

// getPendingHighestBid returns the highest pending bid for an auction
func (bu *BidUseCase) getPendingHighestBid(auctionId string) *bid_entity.Bid {
	bu.pendingHighestBidMutex.RLock()
	defer bu.pendingHighestBidMutex.RUnlock()
	return bu.pendingHighestBid[auctionId]
}

// updatePendingHighestBid updates the pending highest bid for an auction
func (bu *BidUseCase) updatePendingHighestBid(bid *bid_entity.Bid) {
	bu.pendingHighestBidMutex.Lock()
	defer bu.pendingHighestBidMutex.Unlock()
	bu.pendingHighestBid[bid.AuctionId] = bid
}

func (bu *BidUseCase) CreateBid(
	ctx context.Context,
	bidInputDTO BidInputDTO) *internal_error.InternalError {

	// Validation 1: Create and validate bid entity (amount > 0, valid UUIDs)
	bidEntity, err := bid_entity.CreateBid(bidInputDTO.UserId, bidInputDTO.AuctionId, bidInputDTO.Amount)
	if err != nil {
		return err
	}

	// Validation 2: Check if auction exists and is active
	auction, err := bu.AuctionRepository.FindAuctionById(ctx, bidInputDTO.AuctionId)
	if err != nil {
		return internal_error.NewNotFoundError("Auction not found")
	}
	if auction.Status == auction_entity.Completed {
		return internal_error.NewBadRequestError("Auction is no longer active")
	}

	// Validation 3: Check if user exists
	_, err = bu.UserRepository.FindUserById(ctx, bidInputDTO.UserId)
	if err != nil {
		return internal_error.NewNotFoundError("User not found")
	}

	// Validation 4: Get current highest bid (from DB)
	currentHighestBid, _ := bu.BidRepository.FindWinningBidByAuctionId(ctx, bidInputDTO.AuctionId)

	// Validation 5: Get pending highest bid (from cache - not yet persisted)
	pendingHighestBid := bu.getPendingHighestBid(bidInputDTO.AuctionId)

	// Determine the effective highest bid (max of DB and pending)
	var effectiveHighestAmount float64
	var effectiveHighestUserId string

	if currentHighestBid != nil {
		effectiveHighestAmount = currentHighestBid.Amount
		effectiveHighestUserId = currentHighestBid.UserId
	}

	if pendingHighestBid != nil && pendingHighestBid.Amount > effectiveHighestAmount {
		effectiveHighestAmount = pendingHighestBid.Amount
		effectiveHighestUserId = pendingHighestBid.UserId
	}

	// Validation 6: If there's a highest bid, check constraints
	if effectiveHighestAmount > 0 {
		// Check self-bidding rule (can be enabled via ALLOW_SELF_OUTBID env var)
		if effectiveHighestUserId == bidInputDTO.UserId {
			if !getAllowSelfOutbid() {
				return internal_error.NewBadRequestError("You are already the highest bidder")
			}
		}

		// New bid must be higher than current highest (DB or pending)
		if bidInputDTO.Amount <= effectiveHighestAmount {
			return internal_error.NewBadRequestError("Bid must be higher than current highest bid")
		}
	}

	// Update pending cache BEFORE adding to channel (atomic operation)
	bu.updatePendingHighestBid(bidEntity)

	bu.bidChannel <- *bidEntity

	return nil
}

func getMaxBatchSizeInterval() time.Duration {
	batchInsertInterval := os.Getenv("BATCH_INSERT_INTERVAL")
	duration, err := time.ParseDuration(batchInsertInterval)
	if err != nil {
		return 3 * time.Minute
	}

	return duration
}

func getMaxBatchSize() int {
	value, err := strconv.Atoi(os.Getenv("MAX_BATCH_SIZE"))
	if err != nil {
		return 5
	}

	return value
}

// getAllowSelfOutbid returns whether a user can outbid themselves
// Default: false (user cannot bid if already highest bidder)
// Set ALLOW_SELF_OUTBID=true to allow consecutive bids from same user
func getAllowSelfOutbid() bool {
	value := os.Getenv("ALLOW_SELF_OUTBID")
	return value == "true" || value == "1" || value == "yes"
}
