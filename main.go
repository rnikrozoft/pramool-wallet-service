package main

import (
	"database/sql"
	"log"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	_ "github.com/lib/pq"
	"github.com/rnikrozoft/pramool-wallet-service/handler"
	"github.com/rnikrozoft/pramool-wallet-service/middleware"
	"github.com/rnikrozoft/pramool-wallet-service/repository"
	"github.com/rnikrozoft/pramool-wallet-service/service"
)

func main() {
	dsn := os.Getenv("DATABASE_DSN")
	jwtSecret := os.Getenv("JWT_SECRET")
	webhookSecret := strings.Trim(os.Getenv("OMISE_WEBHOOK_SECRET"), "\"' ")
	port := os.Getenv("PORT")
	if port == "" {
		port = "3102"
	}

	conn, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	app := fiber.New(fiber.Config{
		AppName: "pramool-wallet-service",
	})
	corsOrigins := strings.TrimSpace(os.Getenv("CORS_ALLOW_ORIGINS"))
	if corsOrigins == "" {
		corsOrigins = "http://localhost:3000"
	}
	app.Use(cors.New(cors.Config{
		AllowOrigins:     corsOrigins,
		AllowOriginsFunc: corsAllowDevLAN,
		AllowCredentials: true,
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization, Cookie",
	}))

	walletRepository := repository.NewWalletRepository(conn)
	walletService := service.NewWalletService(os.Getenv("OMISE_SECRET_KEY"), walletRepository)
	walletHandler := handler.NewWalletHandler(walletService, webhookSecret)
	m := middleware.Middleware{JWTSecret: jwtSecret}

	app.Post("/wallet/topup", m.JWTMiddleware, walletHandler.Topup)
	app.Get("/wallet/transactions", m.JWTMiddleware, walletHandler.Transactions)
	app.Post("/webhooks/omise", walletHandler.OmiseWebhook)

	log.Fatal(app.Listen(":" + port))
}
