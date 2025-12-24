package app

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
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
	directDao "github.com/vadim/neo-metric/internal/domain/direct/dao"
	directEntity "github.com/vadim/neo-metric/internal/domain/direct/entity"
	directPolicy "github.com/vadim/neo-metric/internal/domain/direct/policy"
	directScheduler "github.com/vadim/neo-metric/internal/domain/direct/scheduler"
	directService "github.com/vadim/neo-metric/internal/domain/direct/service"
	"github.com/vadim/neo-metric/internal/domain/publication/dao"
	"github.com/vadim/neo-metric/internal/domain/publication/policy"
	publicationScheduler "github.com/vadim/neo-metric/internal/domain/publication/scheduler"
	"github.com/vadim/neo-metric/internal/domain/publication/service"
	templateDao "github.com/vadim/neo-metric/internal/domain/template/dao"
	templateEntity "github.com/vadim/neo-metric/internal/domain/template/entity"
	templatePolicy "github.com/vadim/neo-metric/internal/domain/template/policy"
	templateService "github.com/vadim/neo-metric/internal/domain/template/service"
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
	directPolicy      *directPolicy.Policy
	templatePolicy    *templatePolicy.Policy

	// Services for sync schedulers
	commentService *commentService.Service
	directService  *directService.Service

	// Account lister for HTTP handlers
	accountLister *accountListerAdapter

	// Publication repository for comment sync
	publicationRepo dao.PublicationRepository

	// Scheduler for processing scheduled publications
	scheduler *publicationScheduler.Scheduler

	// Comment sync scheduler
	commentSyncScheduler *commentScheduler.Scheduler

	// Direct message sync scheduler
	directSyncScheduler *directScheduler.Scheduler
}

// parseLogLevel converts string log level to slog.Level
func parseLogLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// NewApp creates and initializes the application
func NewApp(ctx context.Context, cfg config.Config) (*App, error) {
	// Initialize logger with configurable level
	logLevel := parseLogLevel(cfg.Logger.Level)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
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

		// Initialize direct message sync scheduler
		if app.directService != nil && app.pg != nil {
			app.directSyncScheduler = directScheduler.New(
				app.directService,
				&accountProviderAdapter{dao.NewAccountPostgres(app.pg)},
				directScheduler.Config{
					Interval:  cfg.Scheduler.DirectSyncInterval,
					SyncAge:   cfg.Scheduler.DirectSyncAge,
					BatchSize: cfg.Scheduler.DirectSyncBatchSize,
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
		instagram.WithLogger(a.logger),
	)
	igPublisher := instagram.NewPublisher(igClient)

	// Initialize DAOs
	var publicationsRepo dao.PublicationRepository
	var mediaRepo dao.MediaRepository
	var accountProvider policy.AccountProvider
	var commentRepo commentService.CommentRepository
	var commentSyncRepo commentService.SyncStatusRepository

	// Direct message repositories
	var directConvRepo directService.ConversationRepository
	var directMsgRepo directService.MessageRepository
	var directConvSyncRepo directService.ConversationSyncRepository
	var directAccountSyncRepo directService.AccountSyncRepository

	// Template repository
	var templateRepo templateService.TemplateRepository

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

		// Direct message repositories
		directConvRepo = &directConvRepoAdapter{directDao.NewConversationPostgres(a.pg)}
		directMsgRepo = &directMsgRepoAdapter{directDao.NewMessagePostgres(a.pg)}
		directConvSyncRepo = &directConvSyncRepoAdapter{directDao.NewConversationSyncPostgres(a.pg)}
		directAccountSyncRepo = &directAccountSyncRepoAdapter{directDao.NewAccountSyncPostgres(a.pg)}

		// Template repository
		templateRepo = &templateRepoAdapter{templateDao.NewTemplatePostgres(a.pg)}
	}

	// Initialize publication service
	pubService := service.New(publicationsRepo, mediaRepo)

	// Initialize publication policy
	a.publicationPolicy = policy.New(pubService, &instagramPublisherAdapter{igPublisher}, accountProvider)

	// Initialize comment domain
	igCommentAdapter := &instagramCommentAdapter{igClient}
	if commentRepo != nil && commentSyncRepo != nil {
		a.commentService = commentService.NewWithRepo(igCommentAdapter, commentRepo, commentSyncRepo).
			WithSyncMaxAge(a.cfg.Scheduler.CommentCacheMaxAge)
	} else {
		a.commentService = commentService.New(igCommentAdapter).
			WithSyncMaxAge(a.cfg.Scheduler.CommentCacheMaxAge)
	}
	a.commentPolicy = commentPolicy.New(a.commentService, accountProvider)

	// Initialize direct message domain
	igDirectAdapter := &instagramDirectAdapter{igClient}
	if directConvRepo != nil && directMsgRepo != nil {
		a.directService = directService.NewWithRepo(
			igDirectAdapter,
			directConvRepo,
			directMsgRepo,
			directConvSyncRepo,
			directAccountSyncRepo,
		)
	} else {
		a.directService = directService.New(igDirectAdapter)
	}
	a.directPolicy = directPolicy.New(a.directService, accountProvider)

	// Wire DirectSender for send_to_direct functionality
	if a.directService != nil && accountProvider != nil {
		directSender := &directSenderAdapter{
			directSvc: a.directService,
			accounts:  accountProvider,
		}
		a.commentPolicy = a.commentPolicy.WithDirectSender(directSender)
	}

	// Initialize template domain
	if templateRepo != nil {
		tmplService := templateService.New(templateRepo)
		a.templatePolicy = templatePolicy.New(tmplService)
	}

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

		// Direct message routes
		if a.directPolicy != nil {
			directHandler := httpcontroller.NewDirectHandler(a.directPolicy)
			directHandler.RegisterRoutes(r)
		}

		// Template routes
		if a.templatePolicy != nil {
			templateHandler := httpcontroller.NewTemplateHandler(a.templatePolicy)
			templateHandler.RegisterRoutes(r)
		}

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

	// Start direct message sync scheduler if enabled
	if a.directSyncScheduler != nil {
		go a.directSyncScheduler.Start(ctx)
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

	// Stop direct message sync scheduler
	if a.directSyncScheduler != nil {
		a.directSyncScheduler.Stop()
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
		var timestamp time.Time
		if c.Timestamp != "" {
			// Instagram uses format "2025-12-24T07:53:58+0000", try multiple formats
			for _, layout := range []string{
				"2006-01-02T15:04:05-0700",
				"2006-01-02T15:04:05Z0700",
				time.RFC3339,
			} {
				if t, err := time.Parse(layout, c.Timestamp); err == nil {
					timestamp = t
					break
				}
			}
		}

		comments[i] = commentEntity.Comment{
			ID:           c.ID,
			MediaID:      mediaID,
			Username:     c.Username,
			Text:         c.Text,
			Timestamp:    timestamp,
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
		var timestamp time.Time
		if c.Timestamp != "" {
			for _, layout := range []string{
				"2006-01-02T15:04:05-0700",
				"2006-01-02T15:04:05Z0700",
				time.RFC3339,
			} {
				if t, err := time.Parse(layout, c.Timestamp); err == nil {
					timestamp = t
					break
				}
			}
		}

		comments[i] = commentEntity.Comment{
			ID:        c.ID,
			ParentID:  commentID,
			Username:  c.Username,
			Text:      c.Text,
			Timestamp: timestamp,
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

func (a *commentRepoAdapter) GetStatistics(ctx context.Context, accountID string, topPostsLimit int) (*commentEntity.CommentStatistics, error) {
	return a.repo.GetStatistics(ctx, accountID, topPostsLimit)
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

// instagramDirectAdapter adapts instagram.Client to directService.InstagramClient
type instagramDirectAdapter struct {
	client *instagram.Client
}

func (a *instagramDirectAdapter) GetConversations(ctx context.Context, userID, accessToken string, limit int, after string) (*directService.ConversationsResult, error) {
	out, err := a.client.GetDMConversations(ctx, instagram.GetDMConversationsInput{
		UserID:      userID,
		AccessToken: accessToken,
		Limit:       limit,
		After:       after,
	})
	if err != nil {
		return nil, err
	}

	// Debug: log raw API response structure (commented out for production)
	// log.Printf("[DEBUG] GetConversations raw response: Data=%d items, Paging=%+v", len(out.Data), out.Paging)
	// for i, c := range out.Data {
	// 	log.Printf("[DEBUG]   Data[%d]: ID=%s, UpdatedTime=%s, Participants=%+v, Messages=%+v",
	// 		i, c.ID, c.UpdatedTime, c.Participants, c.Messages)
	// }

	conversations := make([]directEntity.Conversation, len(out.Data))
	for i, c := range out.Data {
		var lastMessageAt *time.Time
		if c.UpdatedTime != "" {
			// Instagram uses format "2024-02-06T13:41:22+0000", try multiple formats
			for _, layout := range []string{
				"2006-01-02T15:04:05-0700",
				"2006-01-02T15:04:05Z0700",
				time.RFC3339,
			} {
				if t, err := time.Parse(layout, c.UpdatedTime); err == nil {
					lastMessageAt = &t
					break
				}
			}
		}

		conv := directEntity.Conversation{
			ID:            c.ID,
			LastMessageAt: lastMessageAt,
		}

		// Extract participant info - find the OTHER party (not the owner)
		if c.Participants != nil {
			for _, p := range c.Participants.Data {
				if p.ID != userID { // Skip the owner, take the other party
					conv.ParticipantID = p.ID
					conv.ParticipantUsername = p.Username
					conv.ParticipantName = p.Name
					break
				}
			}
		}

		// Extract last message info
		if c.Messages != nil && len(c.Messages.Data) > 0 {
			lastMsg := c.Messages.Data[0]
			conv.LastMessageText = lastMsg.Message

			// Check if last message is from the owner
			if lastMsg.From != nil {
				conv.LastMessageIsFromMe = lastMsg.From.ID == userID
			}
		}

		conversations[i] = conv
	}

	var nextCursor string
	hasMore := false
	if out.Paging != nil {
		nextCursor = out.Paging.Cursors.After
		hasMore = out.Paging.Next != ""
	}

	return &directService.ConversationsResult{
		Conversations: conversations,
		NextCursor:    nextCursor,
		HasMore:       hasMore,
	}, nil
}

func (a *instagramDirectAdapter) GetMessages(ctx context.Context, conversationID, userID, accessToken string, limit int, after string) (*directService.MessagesResult, error) {
	out, err := a.client.GetDMMessages(ctx, instagram.GetDMMessagesInput{
		ConversationID: conversationID,
		AccessToken:    accessToken,
		Limit:          limit,
		After:          after,
	})
	if err != nil {
		return nil, err
	}

	messages := make([]directEntity.Message, 0, len(out.Data))
	for _, m := range out.Data {
		// Skip messages without text and without attachments (unsupported content)
		hasAttachments := m.Attachments != nil && len(m.Attachments.Data) > 0
		if m.Message == "" && !hasAttachments {
			continue
		}

		var timestamp time.Time
		if m.CreatedTime != "" {
			// Instagram uses format "2024-02-06T13:41:22+0000", try multiple formats
			for _, layout := range []string{
				"2006-01-02T15:04:05-0700",
				"2006-01-02T15:04:05Z0700",
				time.RFC3339,
			} {
				if t, err := time.Parse(layout, m.CreatedTime); err == nil {
					timestamp = t
					break
				}
			}
		}

		msg := directEntity.Message{
			ID:             m.ID,
			ConversationID: conversationID,
			Text:           m.Message,
			Timestamp:      timestamp,
		}

		if m.From != nil {
			msg.SenderID = m.From.ID
			// Check if message is from the account owner
			msg.IsFromMe = m.From.ID == userID
		}

		// Determine message type from attachments and content
		if hasAttachments {
			att := m.Attachments.Data[0]
			switch {
			case att.ImageData != nil:
				msg.Type = directEntity.MessageTypeImage
				msg.MediaURL = att.ImageData.URL
				msg.MediaType = "image"
			case att.VideoData != nil:
				msg.Type = directEntity.MessageTypeVideo
				msg.MediaURL = att.VideoData.URL
				msg.MediaType = "video"
			case att.Type == "share" || att.ShareURL != "":
				msg.Type = directEntity.MessageTypeShare
				msg.MediaURL = att.ShareURL
			case att.Type == "audio":
				msg.Type = directEntity.MessageTypeAudio
			case att.Type == "story_mention":
				msg.Type = directEntity.MessageTypeStoryMention
			default:
				// Unknown attachment type - skip
				continue
			}
		} else {
			msg.Type = directEntity.MessageTypeText
		}

		messages = append(messages, msg)
	}

	var nextCursor string
	hasMore := false
	if out.Paging != nil {
		nextCursor = out.Paging.Cursors.After
		hasMore = out.Paging.Next != ""
	}

	return &directService.MessagesResult{
		Messages:   messages,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

func (a *instagramDirectAdapter) SendMessage(ctx context.Context, userID, recipientID, accessToken, message string) (*directService.SendMessageResult, error) {
	out, err := a.client.SendDMMessage(ctx, instagram.SendDMMessageInput{
		UserID:      userID,
		RecipientID: recipientID,
		AccessToken: accessToken,
		Message:     message,
	})
	if err != nil {
		return nil, err
	}
	return &directService.SendMessageResult{MessageID: out.MessageID}, nil
}

func (a *instagramDirectAdapter) SendMediaMessage(ctx context.Context, userID, recipientID, accessToken, mediaURL, mediaType string) (*directService.SendMessageResult, error) {
	out, err := a.client.SendDMMediaMessage(ctx, instagram.SendDMMediaMessageInput{
		UserID:      userID,
		RecipientID: recipientID,
		AccessToken: accessToken,
		MediaURL:    mediaURL,
		MediaType:   mediaType,
	})
	if err != nil {
		return nil, err
	}
	return &directService.SendMessageResult{MessageID: out.MessageID}, nil
}

func (a *instagramDirectAdapter) GetParticipant(ctx context.Context, userID, accessToken string) (*directService.ParticipantResult, error) {
	out, err := a.client.GetDMParticipant(ctx, instagram.GetDMParticipantInput{
		UserID:      userID,
		AccessToken: accessToken,
	})
	if err != nil {
		return nil, err
	}
	return &directService.ParticipantResult{
		ID:             out.ID,
		Username:       out.Username,
		Name:           out.Name,
		FollowersCount: out.FollowersCount,
	}, nil
}

// directConvRepoAdapter adapts directDao.ConversationPostgres to directService.ConversationRepository
type directConvRepoAdapter struct {
	repo *directDao.ConversationPostgres
}

func (a *directConvRepoAdapter) Upsert(ctx context.Context, conv *directEntity.Conversation) error {
	return a.repo.Upsert(ctx, conv)
}

func (a *directConvRepoAdapter) UpsertBatch(ctx context.Context, convs []directEntity.Conversation) error {
	return a.repo.UpsertBatch(ctx, convs)
}

func (a *directConvRepoAdapter) GetByID(ctx context.Context, id string) (*directEntity.Conversation, error) {
	return a.repo.GetByID(ctx, id)
}

func (a *directConvRepoAdapter) GetByAccountID(ctx context.Context, accountID string, limit, offset int) ([]directEntity.Conversation, error) {
	return a.repo.GetByAccountID(ctx, accountID, limit, offset)
}

func (a *directConvRepoAdapter) Search(ctx context.Context, accountID, query string, limit, offset int) ([]directEntity.Conversation, error) {
	return a.repo.Search(ctx, accountID, query, limit, offset)
}

func (a *directConvRepoAdapter) Delete(ctx context.Context, id string) error {
	return a.repo.Delete(ctx, id)
}

func (a *directConvRepoAdapter) Count(ctx context.Context, accountID string) (int64, error) {
	return a.repo.Count(ctx, accountID)
}

// directMsgRepoAdapter adapts directDao.MessagePostgres to directService.MessageRepository
type directMsgRepoAdapter struct {
	repo *directDao.MessagePostgres
}

func (a *directMsgRepoAdapter) Upsert(ctx context.Context, msg *directEntity.Message) error {
	return a.repo.Upsert(ctx, msg)
}

func (a *directMsgRepoAdapter) UpsertBatch(ctx context.Context, msgs []directEntity.Message) error {
	return a.repo.UpsertBatch(ctx, msgs)
}

func (a *directMsgRepoAdapter) GetByID(ctx context.Context, id string) (*directEntity.Message, error) {
	return a.repo.GetByID(ctx, id)
}

func (a *directMsgRepoAdapter) GetByConversationID(ctx context.Context, conversationID string, limit, offset int) ([]directEntity.Message, error) {
	return a.repo.GetByConversationID(ctx, conversationID, limit, offset)
}

func (a *directMsgRepoAdapter) Delete(ctx context.Context, id string) error {
	return a.repo.Delete(ctx, id)
}

func (a *directMsgRepoAdapter) Count(ctx context.Context, conversationID string) (int64, error) {
	return a.repo.Count(ctx, conversationID)
}

func (a *directMsgRepoAdapter) GetStatistics(ctx context.Context, filter directEntity.StatisticsFilter) (*directEntity.Statistics, error) {
	return a.repo.GetStatistics(ctx, filter)
}

func (a *directMsgRepoAdapter) GetHeatmap(ctx context.Context, filter directEntity.StatisticsFilter) (*directEntity.Heatmap, error) {
	return a.repo.GetHeatmap(ctx, filter)
}

// directConvSyncRepoAdapter adapts directDao.ConversationSyncPostgres to directService.ConversationSyncRepository
type directConvSyncRepoAdapter struct {
	repo *directDao.ConversationSyncPostgres
}

func (a *directConvSyncRepoAdapter) GetSyncStatus(ctx context.Context, conversationID string) (*directService.ConversationSyncStatus, error) {
	status, err := a.repo.GetSyncStatus(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	if status == nil {
		return nil, nil
	}
	return &directService.ConversationSyncStatus{
		ConversationID:         status.ConversationID,
		LastSyncedAt:           status.LastSyncedAt,
		NextCursor:             status.NextCursor,
		SyncComplete:           status.SyncComplete,
		OldestMessageTimestamp: status.OldestMessageTimestamp,
	}, nil
}

func (a *directConvSyncRepoAdapter) UpdateSyncStatus(ctx context.Context, status *directService.ConversationSyncStatus) error {
	return a.repo.UpdateSyncStatus(ctx, &directDao.ConversationSyncStatus{
		ConversationID:         status.ConversationID,
		LastSyncedAt:           status.LastSyncedAt,
		NextCursor:             status.NextCursor,
		SyncComplete:           status.SyncComplete,
		OldestMessageTimestamp: status.OldestMessageTimestamp,
	})
}

func (a *directConvSyncRepoAdapter) GetConversationsNeedingSync(ctx context.Context, accountID string, olderThan time.Duration, limit int) ([]string, error) {
	return a.repo.GetConversationsNeedingSync(ctx, accountID, olderThan, limit)
}

// directAccountSyncRepoAdapter adapts directDao.AccountSyncPostgres to directService.AccountSyncRepository
type directAccountSyncRepoAdapter struct {
	repo *directDao.AccountSyncPostgres
}

func (a *directAccountSyncRepoAdapter) GetSyncStatus(ctx context.Context, accountID string) (*directService.AccountSyncStatus, error) {
	status, err := a.repo.GetSyncStatus(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if status == nil {
		return nil, nil
	}
	return &directService.AccountSyncStatus{
		AccountID:    status.AccountID,
		LastSyncedAt: status.LastSyncedAt,
		NextCursor:   status.NextCursor,
		SyncComplete: status.SyncComplete,
	}, nil
}

func (a *directAccountSyncRepoAdapter) UpdateSyncStatus(ctx context.Context, status *directService.AccountSyncStatus) error {
	return a.repo.UpdateSyncStatus(ctx, &directDao.AccountSyncStatus{
		AccountID:    status.AccountID,
		LastSyncedAt: status.LastSyncedAt,
		NextCursor:   status.NextCursor,
		SyncComplete: status.SyncComplete,
	})
}

func (a *directAccountSyncRepoAdapter) GetAccountsNeedingSync(ctx context.Context, olderThan time.Duration, limit int) ([]string, error) {
	return a.repo.GetAccountsNeedingSync(ctx, olderThan, limit)
}

// templateRepoAdapter adapts templateDao.TemplatePostgres to templateService.TemplateRepository
type templateRepoAdapter struct {
	repo *templateDao.TemplatePostgres
}

func (a *templateRepoAdapter) Create(ctx context.Context, tmpl *templateEntity.Template) error {
	return a.repo.Create(ctx, tmpl)
}

func (a *templateRepoAdapter) GetByID(ctx context.Context, id string) (*templateEntity.Template, error) {
	return a.repo.GetByID(ctx, id)
}

func (a *templateRepoAdapter) Update(ctx context.Context, tmpl *templateEntity.Template) error {
	return a.repo.Update(ctx, tmpl)
}

func (a *templateRepoAdapter) Delete(ctx context.Context, id string) error {
	return a.repo.Delete(ctx, id)
}

func (a *templateRepoAdapter) List(ctx context.Context, filter templateService.ListFilter, opts templateService.ListOptions) ([]templateEntity.Template, error) {
	return a.repo.List(ctx, templateDao.ListFilter{
		AccountID: filter.AccountID,
		Type:      filter.Type,
	}, templateDao.ListOptions{
		Limit:  opts.Limit,
		Offset: opts.Offset,
		SortBy: opts.SortBy,
		Desc:   opts.Desc,
	})
}

func (a *templateRepoAdapter) Count(ctx context.Context, filter templateService.ListFilter) (int64, error) {
	return a.repo.Count(ctx, templateDao.ListFilter{
		AccountID: filter.AccountID,
		Type:      filter.Type,
	})
}

func (a *templateRepoAdapter) IncrementUsageCount(ctx context.Context, id string) error {
	return a.repo.IncrementUsageCount(ctx, id)
}

// directSenderAdapter adapts directService to commentPolicy.DirectSender
type directSenderAdapter struct {
	directSvc *directService.Service
	accounts  policy.AccountProvider
}

func (a *directSenderAdapter) SendMessage(ctx context.Context, accountID, recipientID, message string) error {
	// Get access token and Instagram user ID for this account
	accessToken, err := a.accounts.GetAccessToken(ctx, accountID)
	if err != nil {
		return fmt.Errorf("getting access token: %w", err)
	}

	userID, err := a.accounts.GetInstagramUserID(ctx, accountID)
	if err != nil {
		return fmt.Errorf("getting Instagram user ID: %w", err)
	}

	// Send the DM
	_, err = a.directSvc.SendMessage(ctx, directService.SendMessageInput{
		UserID:      userID,
		RecipientID: recipientID,
		AccessToken: accessToken,
		Message:     message,
	})
	return err
}
