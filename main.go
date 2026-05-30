package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gofiber/contrib/otelfiber"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/rnikrozoft/pramool-wallet-service/handler"
	"github.com/rnikrozoft/pramool-wallet-service/internal/config"
	"github.com/rnikrozoft/pramool-wallet-service/internal/telemetry"
	"github.com/rnikrozoft/pramool-wallet-service/middleware"
	"github.com/rnikrozoft/pramool-wallet-service/repository"
	"github.com/rnikrozoft/pramool-wallet-service/service"
	"go.uber.org/zap"
)

func main() {
	_ = godotenv.Load()
	logger, err := telemetry.NewZapLogger()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	shutdownTelemetry, err := telemetry.Init("pramool-wallet-service")
	if err != nil {
		logger.Fatal("otel init", zap.Error(err))
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownTelemetry(ctx); err != nil {
			logger.Warn("otel shutdown", zap.Error(err))
		}
	}()

	dsn, err := postgresDSN()
	if err != nil {
		logger.Fatal("database config", zap.Error(err))
	}
	jwtSecret := os.Getenv("JWT_SECRET")
	webhookSecret := strings.Trim(os.Getenv("OMISE_WEBHOOK_SECRET"), "\"' ")
	port := os.Getenv("PORT")
	if port == "" {
		port = "3102"
	}

	conn, err := sql.Open("postgres", dsn)
	if err != nil {
		logger.Fatal("database open", zap.Error(err))
	}
	defer conn.Close()

	app := fiber.New(fiber.Config{
		AppName: "pramool-wallet-service",
	})
	app.Use(otelfiber.Middleware())
	app.Use(telemetry.AccessLogWithZap(logger))
	telemetry.MountHealth(app, "pramool-wallet-service")
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
	omiseHTTP := telemetry.DefaultHTTPClient(90 * time.Second)
	feesCfg := config.LoadWalletFeesFromEnv()
	walletService := service.NewWalletService(os.Getenv("OMISE_SECRET_KEY"), walletRepository, omiseHTTP, feesCfg)
	walletHandler := handler.NewWalletHandler(walletService, webhookSecret)
	m := middleware.Middleware{JWTSecret: jwtSecret}

	app.Get("/wallet/fees", walletHandler.FeeRates)
	app.Post("/wallet/topup", m.JWTMiddleware, walletHandler.Topup)
	app.Post("/wallet/withdraw", m.JWTMiddleware, walletHandler.Withdraw)
	app.Get("/wallet/transactions", m.JWTMiddleware, walletHandler.Transactions)
	app.Post("/webhooks/omise", walletHandler.OmiseWebhook)

	addr := ":" + port
	go func() {
		if err := app.Listen(addr); err != nil {
			logger.Error("listen stopped", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		logger.Error("fiber shutdown", zap.Error(err))
	}
}

func postgresDSN() (string, error) {
	if dsn := strings.TrimSpace(os.Getenv("DATABASE_DSN")); dsn != "" {
		return dsn, nil
	}
	host := strings.TrimSpace(os.Getenv("DATABASE_HOST"))
	user := strings.TrimSpace(os.Getenv("DATABASE_USERNAME"))
	pass := os.Getenv("DATABASE_PASSWORD")
	name := strings.TrimSpace(os.Getenv("DATABASE_NAME"))
	port := strings.TrimSpace(os.Getenv("DATABASE_PORT"))
	if host == "" || user == "" || name == "" {
		return "", fmt.Errorf("set DATABASE_DSN or DATABASE_HOST, DATABASE_USERNAME, DATABASE_NAME")
	}
	if port == "" {
		port = "5432"
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, pass, host, port, name), nil
}
