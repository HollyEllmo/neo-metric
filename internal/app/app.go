package app

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vadim/neo-metric/internal/config"
	httpcontroller "github.com/vadim/neo-metric/internal/controller/http"
	"github.com/vadim/neo-metric/internal/database"
	commentDao "github.com/vadim/neo-metric/internal/domain/comment/dao"
	commentEntity "github.com/vadim/neo-metric/internal/domain/comment/entity"
	commentPolicy "github.com/vadim/neo-metric/internal/domain/comment/policy"
	commentScheduler "github.com/vadim/neo-metric/internal/domain/comment/scheduler"
	commentService "github.com/vadim/neo-metric/internal/domain/comment/service"
	"github.com/vadim/neo-metric/internal/domain/publication/dao"
	"github.com/vadim/neo-metric/internal/domain/publication/policy"
	publicationScheduler "github.com/vadim/neo-metric/internal/domain/publication/scheduler"
	"github.com/vadim/neo-metric/internal/domain/publication/service"
	"github.com/vadim/neo-metric/internal/httpx/upstream/instagram"
	"github.com/vadim/neo-metric/internal/storage"
)

// App is the main application container
type App struct {
	cfg        config.Config
	httpServer *http.Server
	router     *chi.Mux
	logger     *slog.Logger
	pg         *pgxpool.Pool
	s3         *storage.S3Storage

	// Domain policies (interfaces for HTTP handlers)
	publicationPolicy *policy.Policy
	commentPolicy     *commentPolicy.Policy

	// Comment service for sync scheduler
	commentService *commentService.Service

	// Account lister for HTTP handlers
	accountLister *accountListerAdapter

	// Publication repository for comment sync
	publicationRepo dao.PublicationRepository

	// Scheduler for processing scheduled publications
	scheduler *publicationScheduler.Scheduler

	// Comment sync scheduler
	commentSyncScheduler *commentScheduler.Scheduler
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
		app.scheduler = publicationScheduler.New(app.publicationPolicy, cfg.Scheduler.Interval, logger)

		// Initialize comment sync scheduler if we have the necessary components
		if app.commentService != nil && app.publicationRepo != nil && app.accountLister != nil {
			app.commentSyncScheduler = commentScheduler.New(
				app.commentService,
				&publicationRepoAdapter{app.publicationRepo},
				&accountProviderAdapter{dao.NewAccountPostgres(app.pg)},
				commentScheduler.Config{
					Interval:  cfg.Scheduler.CommentSyncInterval,
					SyncAge:   cfg.Scheduler.CommentSyncAge,
					BatchSize: cfg.Scheduler.CommentSyncBatchSize,
				},
				logger,
			)
		}
	}

	return app, nil
}

// initInfrastructure initializes infrastructure components (DB, Redis, etc.)
func (a *App) initInfrastructure(ctx context.Context) error {
	// Initialize PostgreSQL connection if DSN is provided
	if a.cfg.Database.PostgresDSN != "" {
		pool, err := database.NewPostgresPool(ctx, a.cfg.Database.PostgresDSN)
		if err != nil {
			return fmt.Errorf("connecting to postgres: %w", err)
		}
		a.pg = pool
		a.logger.Info("connected to PostgreSQL")
	}

	// Initialize S3 storage
	if a.cfg.S3.Endpoint != "" {
		s3Storage, err := storage.NewS3Storage(storage.S3Config{
			Endpoint:        a.cfg.S3.Endpoint,
			AccessKeyID:     a.cfg.S3.AccessKeyID,
			SecretAccessKey: a.cfg.S3.SecretAccessKey,
			Bucket:          a.cfg.S3.Bucket,
			Region:          a.cfg.S3.Region,
			PublicURL:       a.cfg.S3.PublicURL,
		})
		if err != nil {
			return fmt.Errorf("initializing s3 storage: %w", err)
		}
		a.s3 = s3Storage
		a.logger.Info("initialized S3 storage", "endpoint", a.cfg.S3.Endpoint)
	}

	return nil
}

// initDomains initializes domain layers (DAO, Service, Policy)
func (a *App) initDomains(_ context.Context) error {
	// Initialize Instagram client
	igClient := instagram.New(
		instagram.WithBaseURL(a.cfg.Instagram.BaseURL),
		instagram.WithAPIVersion(a.cfg.Instagram.APIVersion),
	)
	igPublisher := instagram.NewPublisher(igClient)

	// Initialize DAOs
	var publicationsRepo dao.PublicationRepository
	var mediaRepo dao.MediaRepository
	var accountProvider policy.AccountProvider
	var commentRepo commentService.CommentRepository
	var commentSyncRepo commentService.SyncStatusRepository

	if a.pg != nil {
		// Use PostgreSQL implementations
		publicationsRepo = dao.NewPublicationPostgres(a.pg)
		mediaRepo = dao.NewMediaPostgres(a.pg)
		accountRepo := dao.NewAccountPostgres(a.pg)
		accountProvider = &accountProviderAdapter{accountRepo}
		a.accountLister = &accountListerAdapter{accountRepo}
		a.publicationRepo = publicationsRepo

		// Comment repositories
		commentRepo = &commentRepoAdapter{commentDao.NewCommentPostgres(a.pg)}
		commentSyncRepo = &commentSyncRepoAdapter{commentDao.NewSyncStatusPostgres(a.pg)}
	}

	// Initialize publication service
	pubService := service.New(publicationsRepo, mediaRepo)

	// Initialize publication policy
	a.publicationPolicy = policy.New(pubService, &instagramPublisherAdapter{igPublisher}, accountProvider)

	// Initialize comment domain
	igCommentAdapter := &instagramCommentAdapter{igClient}
	if commentRepo != nil && commentSyncRepo != nil {
		a.commentService = commentService.NewWithRepo(igCommentAdapter, commentRepo, commentSyncRepo)
	} else {
		a.commentService = commentService.New(igCommentAdapter)
	}
	a.commentPolicy = commentPolicy.New(a.commentService, accountProvider)

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

		// Comment routes
		commentHandler := httpcontroller.NewCommentHandler(a.commentPolicy)
		commentHandler.RegisterRoutes(r)

		// Account routes
		if a.accountLister != nil {
			accHandler := httpcontroller.NewAccountHandler(a.accountLister)
			accHandler.RegisterRoutes(r)
		}

		// Media upload routes
		if a.s3 != nil {
			mediaHandler := httpcontroller.NewMediaHandler(&mediaUploaderAdapter{a.s3})
			mediaHandler.RegisterRoutes(r)
		}
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
	w.Header().Set("Content-Type", "application/json")

	// Check database connectivity
	if a.pg != nil {
		if err := a.pg.Ping(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"status":"not ready","error":"database connection failed"}`))
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ready"}`))
}

// Run starts the application and blocks until shutdown signal
func (a *App) Run(ctx context.Context) error {
	// Start scheduler if enabled
	if a.scheduler != nil {
		go a.scheduler.Start(ctx)
	}

	// Start comment sync scheduler if enabled
	if a.commentSyncScheduler != nil {
		go a.commentSyncScheduler.Start(ctx)
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

	// Stop comment sync scheduler
	if a.commentSyncScheduler != nil {
		a.commentSyncScheduler.Stop()
	}

	// Shutdown HTTP server with timeout
	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := a.httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutting down HTTP server: %w", err)
	}

	// Close database connections
	if a.pg != nil {
		a.pg.Close()
	}

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

// accountProviderAdapter adapts AccountPostgres to policy.AccountProvider
type accountProviderAdapter struct {
	repo *dao.AccountPostgres
}

func (a *accountProviderAdapter) GetAccessToken(ctx context.Context, accountID string) (string, error) {
	return a.repo.GetAccessToken(ctx, accountID)
}

func (a *accountProviderAdapter) GetInstagramUserID(ctx context.Context, accountID string) (string, error) {
	return a.repo.GetInstagramUserID(ctx, accountID)
}

// accountListerAdapter adapts AccountPostgres to httpcontroller.AccountLister
type accountListerAdapter struct {
	repo *dao.AccountPostgres
}

func (a *accountListerAdapter) ListAccounts(ctx context.Context) ([]httpcontroller.AccountInfo, error) {
	accounts, err := a.repo.ListAccounts(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]httpcontroller.AccountInfo, len(accounts))
	for i, acc := range accounts {
		result[i] = httpcontroller.AccountInfo{
			ID:              acc.ID,
			InstagramUserID: acc.InstagramUserID,
			Username:        acc.Username,
			HasAccessToken:  acc.AccessToken != "",
		}
	}
	return result, nil
}

// mediaUploaderAdapter adapts S3Storage to httpcontroller.MediaUploader
type mediaUploaderAdapter struct {
	storage *storage.S3Storage
}

func (a *mediaUploaderAdapter) Upload(ctx context.Context, in httpcontroller.MediaUploadInput) (*httpcontroller.MediaUploadOutput, error) {
	out, err := a.storage.Upload(ctx, storage.UploadInput{
		Reader:      in.Reader.(io.Reader),
		ContentType: in.ContentType,
		Size:        in.Size,
		Filename:    in.Filename,
	})
	if err != nil {
		return nil, err
	}
	return &httpcontroller.MediaUploadOutput{
		URL:  out.URL,
		Key:  out.Key,
		Size: out.Size,
	}, nil
}

// instagramCommentAdapter adapts instagram.Client to commentService.InstagramClient
type instagramCommentAdapter struct {
	client *instagram.Client
}

func (a *instagramCommentAdapter) GetComments(ctx context.Context, mediaID, accessToken string, limit int, after string) (*commentService.CommentsResult, error) {
	out, err := a.client.GetComments(ctx, instagram.GetCommentsInput{
		MediaID:     mediaID,
		AccessToken: accessToken,
		Limit:       limit,
		After:       after,
	})
	if err != nil {
		return nil, err
	}

	comments := make([]commentEntity.Comment, len(out.Data))
	for i, c := range out.Data {
		comments[i] = commentEntity.Comment{
			ID:           c.ID,
			MediaID:      mediaID,
			Username:     c.Username,
			Text:         c.Text,
			Timestamp:    c.Timestamp,
			LikeCount:    c.LikeCount,
			IsHidden:     c.Hidden,
			RepliesCount: c.RepliesCount,
		}
	}

	var nextCursor string
	hasMore := false
	if out.Paging != nil {
		nextCursor = out.Paging.Cursors.After
		hasMore = out.Paging.Next != ""
	}

	return &commentService.CommentsResult{
		Comments:   comments,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

func (a *instagramCommentAdapter) GetCommentReplies(ctx context.Context, commentID, accessToken string, limit int, after string) (*commentService.CommentsResult, error) {
	out, err := a.client.GetCommentReplies(ctx, instagram.GetCommentRepliesInput{
		CommentID:   commentID,
		AccessToken: accessToken,
		Limit:       limit,
		After:       after,
	})
	if err != nil {
		return nil, err
	}

	comments := make([]commentEntity.Comment, len(out.Data))
	for i, c := range out.Data {
		comments[i] = commentEntity.Comment{
			ID:        c.ID,
			ParentID:  commentID,
			Username:  c.Username,
			Text:      c.Text,
			Timestamp: c.Timestamp,
			LikeCount: c.LikeCount,
			IsHidden:  c.Hidden,
		}
	}

	var nextCursor string
	hasMore := false
	if out.Paging != nil {
		nextCursor = out.Paging.Cursors.After
		hasMore = out.Paging.Next != ""
	}

	return &commentService.CommentsResult{
		Comments:   comments,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

func (a *instagramCommentAdapter) CreateComment(ctx context.Context, mediaID, accessToken, message string) (string, error) {
	out, err := a.client.CreateComment(ctx, instagram.CreateCommentInput{
		MediaID:     mediaID,
		AccessToken: accessToken,
		Message:     message,
	})
	if err != nil {
		return "", err
	}
	return out.ID, nil
}

func (a *instagramCommentAdapter) ReplyToComment(ctx context.Context, commentID, accessToken, message string) (string, error) {
	out, err := a.client.ReplyToComment(ctx, instagram.ReplyToCommentInput{
		CommentID:   commentID,
		AccessToken: accessToken,
		Message:     message,
	})
	if err != nil {
		return "", err
	}
	return out.ID, nil
}

func (a *instagramCommentAdapter) DeleteComment(ctx context.Context, commentID, accessToken string) error {
	return a.client.DeleteComment(ctx, instagram.DeleteCommentInput{
		CommentID:   commentID,
		AccessToken: accessToken,
	})
}

func (a *instagramCommentAdapter) HideComment(ctx context.Context, commentID, accessToken string, hide bool) error {
	return a.client.HideComment(ctx, instagram.HideCommentInput{
		CommentID:   commentID,
		AccessToken: accessToken,
		Hide:        hide,
	})
}

// commentRepoAdapter adapts commentDao.CommentPostgres to commentService.CommentRepository
type commentRepoAdapter struct {
	repo *commentDao.CommentPostgres
}

func (a *commentRepoAdapter) Upsert(ctx context.Context, comment *commentEntity.Comment) error {
	return a.repo.Upsert(ctx, comment)
}

func (a *commentRepoAdapter) UpsertBatch(ctx context.Context, comments []commentEntity.Comment) error {
	return a.repo.UpsertBatch(ctx, comments)
}

func (a *commentRepoAdapter) GetByID(ctx context.Context, id string) (*commentEntity.Comment, error) {
	return a.repo.GetByID(ctx, id)
}

func (a *commentRepoAdapter) GetByMediaID(ctx context.Context, mediaID string, limit int, offset int) ([]commentEntity.Comment, error) {
	return a.repo.GetByMediaID(ctx, mediaID, limit, offset)
}

func (a *commentRepoAdapter) GetReplies(ctx context.Context, parentID string, limit int, offset int) ([]commentEntity.Comment, error) {
	return a.repo.GetReplies(ctx, parentID, limit, offset)
}

func (a *commentRepoAdapter) Delete(ctx context.Context, id string) error {
	return a.repo.Delete(ctx, id)
}

func (a *commentRepoAdapter) UpdateHidden(ctx context.Context, id string, hidden bool) error {
	return a.repo.UpdateHidden(ctx, id, hidden)
}

func (a *commentRepoAdapter) Count(ctx context.Context, mediaID string) (int64, error) {
	return a.repo.Count(ctx, mediaID)
}

func (a *commentRepoAdapter) CountReplies(ctx context.Context, parentID string) (int64, error) {
	return a.repo.CountReplies(ctx, parentID)
}

// commentSyncRepoAdapter adapts commentDao.SyncStatusPostgres to commentService.SyncStatusRepository
type commentSyncRepoAdapter struct {
	repo *commentDao.SyncStatusPostgres
}

func (a *commentSyncRepoAdapter) GetSyncStatus(ctx context.Context, mediaID string) (*commentService.SyncStatus, error) {
	status, err := a.repo.GetSyncStatus(ctx, mediaID)
	if err != nil {
		return nil, err
	}
	if status == nil {
		return nil, nil
	}
	return &commentService.SyncStatus{
		InstagramMediaID: status.InstagramMediaID,
		LastSyncedAt:     status.LastSyncedAt,
		NextCursor:       status.NextCursor,
		SyncComplete:     status.SyncComplete,
	}, nil
}

func (a *commentSyncRepoAdapter) UpdateSyncStatus(ctx context.Context, status *commentService.SyncStatus) error {
	return a.repo.UpdateSyncStatus(ctx, &commentDao.SyncStatus{
		InstagramMediaID: status.InstagramMediaID,
		LastSyncedAt:     status.LastSyncedAt,
		NextCursor:       status.NextCursor,
		SyncComplete:     status.SyncComplete,
	})
}

func (a *commentSyncRepoAdapter) GetMediaIDsNeedingSync(ctx context.Context, olderThan time.Duration, limit int) ([]string, error) {
	return a.repo.GetMediaIDsNeedingSync(ctx, olderThan, limit)
}

// publicationRepoAdapter adapts dao.PublicationRepository for comment sync scheduler
type publicationRepoAdapter struct {
	repo dao.PublicationRepository
}

func (a *publicationRepoAdapter) GetAccountIDByMediaID(ctx context.Context, mediaID string) (string, error) {
	return a.repo.GetAccountIDByMediaID(ctx, mediaID)
}
