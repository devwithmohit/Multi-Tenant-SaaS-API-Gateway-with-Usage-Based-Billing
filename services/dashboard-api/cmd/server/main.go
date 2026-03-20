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

	"github.com/devwithmohit/billing-system/services/dashboard-api/internal/config"
	"github.com/devwithmohit/billing-system/services/dashboard-api/internal/handlers"
	"github.com/devwithmohit/billing-system/services/dashboard-api/internal/middleware"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

func main() {
	log.Println("🚀 Starting Dashboard API Server...")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("Environment: %s", cfg.Server.Environment)
	log.Printf("Server: %s:%s", cfg.Server.Host, cfg.Server.Port)

	// Connect to database
	db, err := cfg.ConnectDB()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	log.Println("✅ Database connected")

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(db, cfg)
	usageHandler := handlers.NewUsageHandler(db)
	apiKeyHandler := handlers.NewAPIKeyHandler(db)
	invoiceHandler := handlers.NewInvoiceHandler(db)
	webhookHandler := handlers.NewWebhookHandler(db)
	alertHandler := handlers.NewAlertHandler(db)
	memberHandler := handlers.NewMemberHandler(db)
	planHandler := handlers.NewPlanHandler(db)
	gdprHandler := handlers.NewGDPRHandler(db)
	orgHandler := handlers.NewOrganizationHandler(db)

	// Initialize middleware
	auditMiddleware := middleware.NewAuditMiddleware(db)

	// Setup router
	r := chi.NewRouter()

	// Global middleware
	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(chiMiddleware.Logger)
	r.Use(chiMiddleware.Recoverer)
	r.Use(chiMiddleware.Timeout(60 * time.Second))

	// CORS middleware
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.CORS.AllowedOrigins,
		AllowedMethods:   cfg.CORS.AllowedMethods,
		AllowedHeaders:   cfg.CORS.AllowedHeaders,
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           cfg.CORS.MaxAge,
	}))

	// Health check endpoint (no auth required)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","service":"dashboard-api"}`))
	})

	// Public routes (no authentication required)
	r.Route("/api/v1/auth", func(r chi.Router) {
		r.Post("/login", authHandler.Login)
		r.Post("/register", authHandler.Register)
	})

	// Public: list available pricing plans (no auth needed)
	r.Get("/api/v1/plans", planHandler.ListPlans)

	// Protected routes (authentication required)
	r.Route("/api/v1", func(r chi.Router) {
		// Apply tenant context middleware for multi-tenancy
		r.Use(middleware.TenantContextMiddleware(db, cfg))
		// Apply audit logging for all state-changing operations
		r.Use(auditMiddleware.Middleware)

		// Auth endpoints
		r.Get("/auth/validate", authHandler.ValidateToken)
		r.Post("/auth/logout", authHandler.Logout)
		r.Post("/auth/refresh", authHandler.Refresh)

		// Usage endpoints
		r.Route("/usage", func(r chi.Router) {
			r.Get("/current", usageHandler.GetCurrentUsage)
			r.Get("/history", usageHandler.GetUsageHistory)
			r.Get("/metrics", usageHandler.GetUsageByMetric)
			r.Get("/export", usageHandler.ExportUsage)
		})

		// API Key endpoints
		r.Route("/apikeys", func(r chi.Router) {
			r.Get("/", apiKeyHandler.ListAPIKeys)
			r.Post("/", apiKeyHandler.CreateAPIKey)
			r.Get("/{id}", apiKeyHandler.GetAPIKey)
			r.Delete("/{id}", apiKeyHandler.RevokeAPIKey)
			r.Post("/{id}/rotate", apiKeyHandler.RotateAPIKey)
		})

		// Invoice endpoints
		r.Route("/invoices", func(r chi.Router) {
			r.Get("/", invoiceHandler.ListInvoices)
			r.Get("/{id}", invoiceHandler.GetInvoice)
			r.Get("/{id}/pdf", invoiceHandler.GetInvoicePDF)
		})

		// Webhook endpoints (Sprint 5.3)
		r.Route("/webhooks", func(r chi.Router) {
			r.Get("/", webhookHandler.ListWebhooks)
			r.Post("/", webhookHandler.CreateWebhook)
			r.Put("/{id}", webhookHandler.UpdateWebhook)
			r.Delete("/{id}", webhookHandler.DeleteWebhook)
		})

		// Alert endpoints (Sprint 5.4)
		r.Route("/alerts", func(r chi.Router) {
			r.Get("/", alertHandler.ListAlerts)
			r.Post("/", alertHandler.CreateAlert)
			r.Put("/{id}", alertHandler.UpdateAlert)
			r.Delete("/{id}", alertHandler.DeleteAlert)
		})

		// Organization routes (Sprint 5.2, 5.5)
		r.Route("/organizations", func(r chi.Router) {
			r.Put("/plan", planHandler.UpdatePlan)
			r.Route("/members", func(r chi.Router) {
				r.Get("/", memberHandler.ListMembers)
				r.Post("/", memberHandler.InviteMember)
				r.Delete("/{memberId}", memberHandler.RemoveMember)
			})
		})

		// Plan endpoints (Sprint 5.2)
		r.Get("/plan", planHandler.GetCurrentPlan)
		r.Put("/plan", planHandler.UpdatePlan)

		// Organization profile (API Contract §2.7)
		r.Get("/organization", orgHandler.GetOrganization)
		r.Put("/organization", orgHandler.UpdateOrganization)

		// GDPR endpoints (Sprint 5.7)
		r.Route("/gdpr", func(r chi.Router) {
			r.Post("/export", gdprHandler.ExportData)
			r.Delete("/delete", gdprHandler.DeleteAccount)
		})
	})

	// 404 handler
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"Not Found","message":"The requested endpoint does not exist"}`))
	})

	// Create HTTP server
	addr := fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("🌐 Server listening on %s", addr)
		log.Println("📋 Available endpoints:")
		log.Println("  POST   /api/v1/auth/login")
		log.Println("  GET    /api/v1/auth/validate")
		log.Println("  GET    /api/v1/usage/current")
		log.Println("  GET    /api/v1/usage/history")
		log.Println("  GET    /api/v1/usage/metrics")
		log.Println("  GET    /api/v1/apikeys")
		log.Println("  POST   /api/v1/apikeys")
		log.Println("  GET    /api/v1/apikeys/{id}")
		log.Println("  DELETE /api/v1/apikeys/{id}")
		log.Println("  GET    /api/v1/invoices")
		log.Println("  GET    /api/v1/invoices/{id}")
		log.Println("  GET    /api/v1/invoices/{id}/pdf")
		log.Println("")
		log.Println("✅ Dashboard API is ready!")

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("🛑 Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("✅ Server stopped gracefully")
}
