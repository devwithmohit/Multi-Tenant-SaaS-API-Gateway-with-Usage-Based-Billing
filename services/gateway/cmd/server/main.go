package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/saas-gateway/gateway/internal/cache"
	"github.com/saas-gateway/gateway/internal/config"
	"github.com/saas-gateway/gateway/internal/database"
	"github.com/saas-gateway/gateway/internal/events"
	"github.com/saas-gateway/gateway/internal/handler"
	"github.com/saas-gateway/gateway/internal/middleware"
	"github.com/saas-gateway/gateway/internal/ratelimit"

	_ "github.com/lib/pq"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize PostgreSQL
	if cfg.DatabaseURL == "" {
		log.Fatalf("DATABASE_URL environment variable is required")
	}

	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to open database connection: %v", err)
	}
	defer db.Close()

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Test database connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	log.Println("✅ Connected to PostgreSQL")

	// Initialize repository
	repo := database.NewRepository(db)

	// Initialize API key cache
	keyCache := cache.NewAPIKeyCache(15 * time.Minute)
	log.Println("✅ Initialized API key cache (TTL: 15m)")

	// Start background cache refresh
	refreshManager := cache.NewRefreshManager(keyCache, repo, 15*time.Minute)
	go refreshManager.Start()
	defer refreshManager.Stop()

	// Initialize Redis (optional for MVP - graceful degradation)
	var rateLimitMiddleware *middleware.RateLimit
	if cfg.RedisAddr != "" {
		redisClient, err := ratelimit.NewRedisClient(ratelimit.RedisConfig{
			Addr:     cfg.RedisAddr,
			Password: cfg.RedisPassword,
			DB:       cfg.RedisDB,
			PoolSize: 10,
		})
		if err != nil {
			log.Printf("⚠️  Warning: Failed to connect to Redis: %v", err)
			log.Println("⚠️  Rate limiting will be disabled")
		} else {
			log.Println("✅ Connected to Redis for rate limiting")
			limiter := ratelimit.NewRateLimiter(redisClient)
			rateLimitMiddleware = middleware.NewRateLimit(limiter)

			// Defer close
			defer redisClient.Close()
		}
	} else {
		log.Println("⚠️  REDIS_ADDR not set - rate limiting disabled")
	}

	// Initialize Kafka event producer (optional - graceful degradation)
	var eventProducer *events.EventProducer
	eventCfg, err := events.LoadConfig()
	if err != nil {
		log.Printf("⚠️  Warning: Failed to load Kafka config: %v", err)
		log.Println("⚠️  Usage event tracking will be disabled")
	} else if eventCfg.Enabled {
		eventProducer, err = events.NewEventProducer(events.ProducerConfig{
			Brokers:       eventCfg.Brokers,
			Topic:         eventCfg.Topic,
			BatchSize:     eventCfg.BatchSize,
			FlushInterval: eventCfg.FlushInterval,
			BufferSize:    eventCfg.BufferSize,
		})
		if err != nil {
			log.Printf("⚠️  Warning: Failed to create Kafka producer: %v", err)
			log.Println("⚠️  Usage event tracking will be disabled")
		} else {
			log.Println("✅ Connected to Kafka for usage tracking")
			defer eventProducer.Close()
		}
	} else {
		log.Println("ℹ️  Kafka disabled - usage event tracking disabled")
	}

	// Initialize handlers
	healthHandler := handler.NewHealth()
	proxyHandler, err := handler.NewProxy(cfg, eventProducer)
	if err != nil {
		log.Fatalf("Failed to initialize proxy handler: %v", err)
	}

	// Initialize middleware
	authMiddleware := middleware.NewAuth(cfg, keyCache, repo)
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

	// Add rate limiting if Redis is available
	if rateLimitMiddleware != nil {
		apiRouter.Use(rateLimitMiddleware.Middleware)
	}

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
