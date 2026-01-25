package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/saas-gateway/gateway/internal/config"
	"github.com/saas-gateway/gateway/internal/handler"
	"github.com/saas-gateway/gateway/internal/middleware"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize handlers
	healthHandler := handler.NewHealth()
	proxyHandler, err := handler.NewProxy(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize proxy handler: %v", err)
	}

	// Initialize middleware
	authMiddleware := middleware.NewAuth(cfg)
	loggerMiddleware := middleware.NewLogger()
	recoveryMiddleware := middleware.NewRecovery()

	// Setup router
	router := mux.NewRouter()

	// Health check endpoints (no auth required)
	router.HandleFunc("/health", healthHandler.ServeHTTP).Methods("GET")
	router.HandleFunc("/health/ready", healthHandler.Ready).Methods("GET")
	router.HandleFunc("/health/live", healthHandler.Live).Methods("GET")

	// API routes (with authentication)
	apiRouter := router.PathPrefix("/").Subrouter()
	apiRouter.Use(authMiddleware.Middleware)
	apiRouter.PathPrefix("/").Handler(proxyHandler)

	// Apply global middleware (order matters: recovery -> logging -> routes)
	handler := recoveryMiddleware.Middleware(
		loggerMiddleware.Middleware(router),
	)

	// Create HTTP server
	addr := fmt.Sprintf(":%s", cfg.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("🚀 Gateway server starting on http://localhost%s", addr)
		log.Printf("📊 Health check available at http://localhost%s/health", addr)
		log.Printf("🔑 API keys loaded: %d", len(cfg.APIKeys))
		log.Printf("🎯 Backend services: %d", len(cfg.BackendURLs))

		for serviceName, url := range cfg.BackendURLs {
			log.Printf("   - %s → %s", serviceName, url)
		}

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("🛑 Shutting down server...")

	// Graceful shutdown with 30 second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("✅ Server stopped gracefully")
}
