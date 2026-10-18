package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"coworking/internal/application"
	"coworking/internal/infrastructure/memory"
	"coworking/internal/infrastructure/postgres"
	"coworking/internal/infrastructure/rss"
	apphttp "coworking/internal/interfaces/http"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serviceRepo := memory.NewServiceRepository()
	catalog := application.NewServiceCatalog(serviceRepo)

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	if err := postgres.RunMigrations(databaseURL); err != nil {
		log.Fatalf("database migrations: %v", err)
	}

	repository, err := postgres.NewRepository(ctx, databaseURL)
	if err != nil {
		log.Fatalf("connect to PostgreSQL: %v", err)
	}
	defer repository.Close()
	bookings := application.NewBookingService(repository, repository)

	// Новости (Лаба 4): свои новости из БД и внешние RSS-ленты, которые
	// добавляются через страницу /news. Свою ленту публикуем на /news/rss.xml.
	newsService := application.NewNewsService(repository, func(title, url string) application.ExternalNewsSource {
		return rss.NewFeed(title, url)
	}, 5*time.Minute)

	server, err := apphttp.NewServer(catalog,
		apphttp.WithSiteTitle("Место"),
		apphttp.WithBookingService(bookings),
		apphttp.WithNewsService(newsService),
		apphttp.WithNewsPublisher(rss.NewPublisher("Место", "Новости коворкинга «Место»", "ru")),
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	httpAddr := loadHTTPAddr()

	httpServer := &http.Server{
		Addr:              httpAddr,
		Handler:           server.Router(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("http shutdown: %v", err)
		}
	}()

	log.Printf("http server listening on %s", httpAddr)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func loadHTTPAddr() string {
	if addr := os.Getenv("HTTP_ADDR"); addr != "" {
		return addr
	}

	return ":8081"
}
