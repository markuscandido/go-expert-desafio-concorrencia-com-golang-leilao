package main

import (
	"context"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/markuscandido/go-expert-desafio-concorrencia-com-golang-leilao/configuration/database/mongodb"
	"github.com/markuscandido/go-expert-desafio-concorrencia-com-golang-leilao/internal/infra/api/web/controller/auction_controller"
	"github.com/markuscandido/go-expert-desafio-concorrencia-com-golang-leilao/internal/infra/api/web/controller/bid_controller"
	"github.com/markuscandido/go-expert-desafio-concorrencia-com-golang-leilao/internal/infra/api/web/controller/user_controller"
	"github.com/markuscandido/go-expert-desafio-concorrencia-com-golang-leilao/internal/infra/database/auction"
	"github.com/markuscandido/go-expert-desafio-concorrencia-com-golang-leilao/internal/infra/database/bid"
	"github.com/markuscandido/go-expert-desafio-concorrencia-com-golang-leilao/internal/infra/database/user"
	"github.com/markuscandido/go-expert-desafio-concorrencia-com-golang-leilao/internal/usecase/auction_usecase"
	"github.com/markuscandido/go-expert-desafio-concorrencia-com-golang-leilao/internal/usecase/bid_usecase"
	"github.com/markuscandido/go-expert-desafio-concorrencia-com-golang-leilao/internal/usecase/user_usecase"
	"go.mongodb.org/mongo-driver/mongo"
)

func main() {
	ctx := context.Background()

	// Tenta carregar .env de múltiplos paths (container e desenvolvimento local)
	// Não fatal se falhar - variáveis podem vir do ambiente do sistema
	envPaths := []string{
		"/cmd/auction/.env", // Path no container scratch
		"cmd/auction/.env",  // Path relativo (desenvolvimento local)
		".env",              // Path alternativo
	}

	envLoaded := false
	for _, path := range envPaths {
		if err := godotenv.Load(path); err == nil {
			envLoaded = true
			log.Printf("Loaded environment from: %s", path)
			break
		}
	}

	if !envLoaded {
		log.Println("No .env file found, using system environment variables")
	}

	databaseConnection, err := mongodb.NewMongoDBConnection(ctx)
	if err != nil {
		log.Fatal(err.Error())
		return
	}

	router := gin.Default()

	userController, bidController, auctionsController, auctionRepo := initDependencies(databaseConnection)

	// Start background goroutine to auto-close expired auctions
	auctionRepo.StartAuctionCloserRoutine(ctx)

	router.GET("/auction", auctionsController.FindAuctions)
	router.GET("/auction/:auctionId", auctionsController.FindAuctionById)
	router.POST("/auction", auctionsController.CreateAuction)
	router.GET("/auction/winner/:auctionId", auctionsController.FindWinningBidByAuctionId)
	router.POST("/bid", bidController.CreateBid)
	router.GET("/bid/:auctionId", bidController.FindBidByAuctionId)
	router.GET("/user/:userId", userController.FindUserById)

	router.Run(":8080")
}

func initDependencies(database *mongo.Database) (
	userController *user_controller.UserController,
	bidController *bid_controller.BidController,
	auctionController *auction_controller.AuctionController,
	auctionRepository *auction.AuctionRepository) {

	auctionRepository = auction.NewAuctionRepository(database)
	bidRepository := bid.NewBidRepository(database, auctionRepository)
	userRepository := user.NewUserRepository(database)

	userController = user_controller.NewUserController(
		user_usecase.NewUserUseCase(userRepository))
	auctionController = auction_controller.NewAuctionController(
		auction_usecase.NewAuctionUseCase(auctionRepository, bidRepository))
	bidController = bid_controller.NewBidController(
		bid_usecase.NewBidUseCase(bidRepository, auctionRepository, userRepository))

	return
}
