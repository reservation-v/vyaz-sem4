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
	apphttp "coworking/internal/interfaces/http"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	repo := memory.NewServiceRepository()
	catalog := application.NewServiceCatalog(repo)

	server, err := apphttp.NewServer(catalog,
		apphttp.WithSiteTitle("Место"),
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
