package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Mansurkhon/go-pos/internal/handler"
	"github.com/Mansurkhon/go-pos/internal/migrate"
	"github.com/Mansurkhon/go-pos/internal/repository"
	"github.com/Mansurkhon/go-pos/internal/service"
	"github.com/Mansurkhon/go-pos/migrations"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return errors.New("DATABASE_URL is not set")
	}
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}

	// ctx отменится по Ctrl+C или SIGTERM — это сигнал к graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return err
	}

	if err := migrate.Up(ctx, pool, migrations.FS); err != nil {
		return err
	}

	svc := service.New(repository.NewPostgres(pool))

	app := fiber.New(fiber.Config{ErrorHandler: handler.ErrorHandler})
	app.Use(recover.New(), logger.New())
	handler.Register(app, svc)

	// Listen блокирует, поэтому запускаем его в goroutine и ждём,
	// что случится раньше: ошибка сервера или сигнал остановки.
	errCh := make(chan error, 1)
	go func() { errCh <- app.Listen(addr) }()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Println("shutting down...")
		return app.ShutdownWithTimeout(5 * time.Second)
	}
}
