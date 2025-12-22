package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/vadim/neo-metric/internal/config"
	httpcontroller "github.com/vadim/neo-metric/internal/controller/http"
	"github.com/vadim/neo-metric/internal/domain/publication/dao"
	"github.com/vadim/neo-metric/internal/domain/publication/policy"
	"github.com/vadim/neo-metric/internal/domain/publication/service"
	"github.com/vadim/neo-metric/internal/httpx/upstream/instagram"
)

// App is the main application container
type App struct {
	cfg        config.Config
	httpServer *http.Server
	router     *chi.Mux
	logger     *slog.Logger

	// Domain policies (interfaces for HTTP handlers)
	publicationPolicy *policy.Policy

	// Scheduler for processing scheduled publications
	scheduler *Scheduler
}

// NewApp creates and initializes the application
func NewApp(ctx context.Context, cfg config.Config) (*App, error) {
	// Initialize logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Initialize router with middleware
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)
	r.Use(middleware.Timeout(30 * time.Second))

	app := &App{
		cfg:    cfg,
		router: r,
		logger: logger,
	}

	// Initialize infrastructure
	if err := app.initInfrastructure(ctx); err != nil {
		return nil, fmt.Errorf("initializing infrastructure: %w", err)
	}

	// Initialize domain layers
	if err := app.initDomains(ctx); err != nil {
		return nil, fmt.Errorf("initializing domains: %w", err)
	}

	// Register routes
	app.registerRoutes()

	// Initialize HTTP server
	app.httpServer = &http.Server{
		Addr:         cfg.Server.Address(),
		Handler:      app.router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// Initialize scheduler
	if cfg.Scheduler.Enabled {
		app.scheduler = NewScheduler(app.publicationPolicy, cfg.Scheduler.Interval, logger)
	}

	return app, nil
}

// initInfrastructure initializes infrastructure components (DB, Redis, etc.)
func (a *App) initInfrastructure(ctx context.Context) error {
	// TODO: Initialize database connection when DBMS is chosen
	// Example:
	// pgClient, err := database.NewPostgresClient(ctx, a.cfg.Database.PostgresDSN)
	// if err != nil {
	//     return fmt.Errorf("connecting to postgres: %w", err)
	// }
	// a.pg = pgClient

	return nil
}

// initDomains initializes domain layers (DAO, Service, Policy)
func (a *App) initDomains(ctx context.Context) error {
	// Initialize Instagram client
	igClient := instagram.New(
		instagram.WithBaseURL(a.cfg.Instagram.BaseURL),
		instagram.WithAPIVersion(a.cfg.Instagram.APIVersion),
	)
	igPublisher := instagram.NewPublisher(igClient)

	// TODO: Replace with real DAO implementations when DBMS is chosen
	// For now, these are nil - will cause panic if used
	// In production, inject real implementations:
	//   publicationsDAO := dao.NewPublicationsPostgres(pgClient)
	//   mediaDAO := dao.NewMediaPostgres(pgClient)
	var publicationsRepo dao.PublicationRepository
	var mediaRepo dao.MediaRepository

	// Initialize service
	pubService := service.New(publicationsRepo, mediaRepo)

	// TODO: Replace with real account provider
	// accountProvider := accountcache.New(laravelClient)
	var accountProvider policy.AccountProvider

	// Initialize policy
	a.publicationPolicy = policy.New(pubService, &instagramPublisherAdapter{igPublisher}, accountProvider)

	return nil
}

// registerRoutes registers all HTTP routes
func (a *App) registerRoutes() {
	// Health check
	a.router.Get("/healthz", a.healthHandler)
	a.router.Get("/readyz", a.readyHandler)

	// Swagger UI documentation
	swaggerHandler := httpcontroller.NewSwaggerHandler("Neo-Metric Instagram API", OpenAPISpec)
	swaggerHandler.RegisterRoutes(a.router)

	// API v1
	a.router.Route("/api/v1", func(r chi.Router) {
		// Publication routes
		pubHandler := httpcontroller.NewPublicationHandler(a.publicationPolicy)
		pubHandler.RegisterRoutes(r)
	})
}

// healthHandler handles health check requests
func (a *App) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

// readyHandler handles readiness check requests
func (a *App) readyHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: Check database connectivity, etc.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ready"}`))
}

// Run starts the application and blocks until shutdown signal
func (a *App) Run(ctx context.Context) error {
	// Start scheduler if enabled
	if a.scheduler != nil {
		go a.scheduler.Start(ctx)
	}

	// Channel to receive errors from server
	errCh := make(chan error, 1)

	// Start HTTP server in goroutine
	go func() {
		a.logger.Info("starting HTTP server", "addr", a.cfg.Server.Address())
		if err := a.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	case sig := <-quit:
		a.logger.Info("received shutdown signal", "signal", sig.String())
	case <-ctx.Done():
		a.logger.Info("context cancelled")
	}

	// Graceful shutdown
	return a.Shutdown(context.Background())
}

// Shutdown gracefully shuts down the application
func (a *App) Shutdown(ctx context.Context) error {
	a.logger.Info("shutting down...")

	// Stop scheduler
	if a.scheduler != nil {
		a.scheduler.Stop()
	}

	// Shutdown HTTP server with timeout
	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := a.httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutting down HTTP server: %w", err)
	}

	// TODO: Close database connections
	// if a.pg != nil {
	//     a.pg.Close()
	// }

	a.logger.Info("shutdown complete")
	return nil
}

// instagramPublisherAdapter adapts instagram.Publisher to policy.InstagramPublisher
type instagramPublisherAdapter struct {
	publisher *instagram.Publisher
}

func (a *instagramPublisherAdapter) Publish(ctx context.Context, in policy.PublishInput) (*policy.PublishOutput, error) {
	out, err := a.publisher.Publish(ctx, instagram.PublishInput{
		UserID:      in.UserID,
		AccessToken: in.AccessToken,
		Publication: in.Publication,
	})
	if err != nil {
		return nil, err
	}
	return &policy.PublishOutput{
		InstagramMediaID: out.InstagramMediaID,
		Permalink:        out.Permalink,
	}, nil
}

func (a *instagramPublisherAdapter) Delete(ctx context.Context, mediaID, accessToken string) error {
	return a.publisher.Delete(ctx, mediaID, accessToken)
}
