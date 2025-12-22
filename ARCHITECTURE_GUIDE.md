# Go Clean Architecture Guide

**Версия:** 1.0
**На основе проекта:** able-sc
**Автор:** Vadim

---

## Содержание

1. [Философия и принципы](#1-философия-и-принципы)
2. [Структура директорий](#2-структура-директорий)
3. [Архитектурные слои](#3-архитектурные-слои)
4. [Composition Root](#4-composition-root)
5. [Domain Layer](#5-domain-layer)
6. [Transport Layer](#6-transport-layer)
7. [Infrastructure](#7-infrastructure)
8. [Паттерны и конвенции](#8-паттерны-и-конвенции)
9. [Быстрый старт нового проекта](#9-быстрый-старт-нового-проекта)

---

## 1. Философия и принципы

### 1.1 Clean Architecture

Проект построен на принципах **Clean Architecture** с инверсией зависимостей:

```
┌─────────────────────────────────────────────────────────┐
│           Transport Layer (HTTP Controllers)            │
│         Тонкие адаптеры, парсинг запросов               │
└─────────────────────────┬───────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────┐
│            Policy Layer (Use-case Orchestration)         │
│      Координация, валидация доменных инвариантов        │
└─────────────────────────┬───────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────┐
│             Service Layer (Business Logic)               │
│       Бизнес-операции, координация DAO                  │
└─────────────────────────┬───────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────┐
│               DAO Layer (Data Access)                    │
│      Чистый доступ к данным, БЕЗ бизнес-логики          │
└─────────────────────────┬───────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────┐
│              Infrastructure Layer                        │
│        Postgres, Redis, внешние API клиенты              │
└─────────────────────────────────────────────────────────┘
```

### 1.2 Ключевые принципы

1. **Зависимости направлены внутрь** - внешние слои зависят от внутренних, не наоборот
2. **Интерфейсы определяются потребителем** - интерфейс объявляется там, где используется
3. **Бизнес-логика изолирована** - не зависит от HTTP, БД, внешних API
4. **Composition Root** - все зависимости инжектятся в одной точке (`internal/app/app.go`)
5. **Domain-Driven Structure** - код организован по bounded contexts (доменам)

### 1.3 Anti-patterns (чего избегать)

- ❌ Бизнес-логика в HTTP handlers
- ❌ SQL запросы в service layer
- ❌ Глобальные переменные и синглтоны
- ❌ Прямые зависимости между доменами (только через интерфейсы)
- ❌ Импорт `net/http` в domain слое

---

## 2. Структура директорий

```
project/
├── cmd/
│   ├── api/                    # Точка входа основного сервиса
│   │   └── main.go
│   └── adminctl/               # CLI инструменты
│       └── main.go
│
├── internal/
│   ├── app/                    # Composition Root
│   │   └── app.go              # Wiring, lifecycle, routing
│   │
│   ├── config/                 # Загрузка конфигурации
│   │   └── config.go           # Структуры + MustLoad()
│   │
│   ├── controller/http/        # HTTP handlers (Transport Layer)
│   │   ├── subcontrol.go       # Handlers для основного домена
│   │   ├── admin.go            # Admin API handlers
│   │   └── statistics.go       # Statistics handlers
│   │
│   ├── domain/                 # Доменная логика (Bounded Contexts)
│   │   ├── <domain_name>/      # Каждый домен в своей директории
│   │   │   ├── policy/         # Use-case orchestration
│   │   │   │   └── policy.go
│   │   │   ├── service/        # Business logic
│   │   │   │   └── service.go
│   │   │   └── dao/            # Data access
│   │   │       └── *.go
│   │   ├── partner/
│   │   │   ├── dao/
│   │   │   └── secretcache/    # Domain-specific infrastructure
│   │   ├── admin/
│   │   │   ├── dao/
│   │   │   └── service/
│   │   └── statistics/
│   │       ├── dao/
│   │       └── service/
│   │
│   ├── database/               # Database clients and generated code
│   │   ├── postgres.go         # Postgres client factory
│   │   ├── redis.go            # Redis client factory
│   │   └── sqlc/               # sqlc generated code
│   │       ├── queries.sql     # SQL queries (EDIT THIS)
│   │       └── *.go            # Generated (DO NOT EDIT)
│   │
│   ├── httpx/                  # HTTP utilities
│   │   ├── middleware/         # HTTP middleware
│   │   │   ├── hmac_verify.go
│   │   │   ├── admin_jwt.go
│   │   │   └── cors.go
│   │   ├── response/           # Response helpers
│   │   └── upstream/           # External API clients
│   │       ├── scon/
│   │       └── imeidetails/
│   │
│   ├── store/                  # Storage abstractions
│   │   └── redisx/             # Redis-based stores
│   │       ├── nonce.go
│   │       └── app_state.go
│   │
│   ├── crypto/                 # Cryptographic utilities
│   │   └── hmacsig/
│   │
│   └── logx/                   # Logging utilities
│       └── db_logger.go
│
├── migrations/                 # Goose SQL migrations
│   ├── 001_init.sql
│   └── ...
│
├── pkg/                        # Shared packages (can be imported externally)
│   └── postgresql/             # pgx client interface wrapper
│
├── docs/                       # Documentation
│   └── openapi.yaml
│
├── scripts/                    # Build/deploy scripts
├── Makefile
├── docker-compose.yml
├── Dockerfile
├── sqlc.yaml
└── .env.example
```

### 2.1 Правила именования директорий

| Директория | Назначение | Примеры файлов |
|------------|-----------|----------------|
| `cmd/<name>/` | Entry points (main.go) | `main.go` |
| `internal/app/` | Composition root | `app.go` |
| `internal/config/` | Configuration | `config.go` |
| `internal/controller/http/` | HTTP handlers | `<domain>.go` |
| `internal/domain/<name>/policy/` | Use-case orchestration | `policy.go` |
| `internal/domain/<name>/service/` | Business logic | `service.go` |
| `internal/domain/<name>/dao/` | Data access | `<entity>.go`, `<entity>_sqlc.go` |
| `internal/httpx/middleware/` | HTTP middleware | `<name>.go` |
| `internal/httpx/upstream/<name>/` | External API clients | `client.go` |
| `internal/store/<name>/` | Storage implementations | `<store_type>.go` |
| `migrations/` | SQL migrations | `NNN_description.sql` |

---

## 3. Архитектурные слои

### 3.1 Transport Layer (`internal/controller/http/`)

**Ответственность:**
- Парсинг HTTP запросов (JSON, form-data, query params)
- Валидация формата входных данных
- Вызов Policy/Service слоя
- Формирование HTTP ответов
- Логирование запросов/ответов

**Паттерн Handler:**

```go
type SubcontrolHandler struct {
    Policy SubcontrolPolicy  // Интерфейс, не конкретный тип
}

func NewSubcontrolHandler(policy SubcontrolPolicy) *SubcontrolHandler {
    return &SubcontrolHandler{Policy: policy}
}

// Handler возвращает http.HandlerFunc
func (h *SubcontrolHandler) Attach() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // 1. Parse request
        var in AttachRequest
        if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
            resp.Error(w, http.StatusBadRequest, "bad json")
            return
        }

        // 2. Call policy (business logic)
        out, err := h.Policy.Attach(r.Context(), policy.AttachInput{
            PartnerID: r.Header.Get("X-Project-Id"),
            Devices:   in.Devices,
        })
        if err != nil {
            resp.Error(w, http.StatusBadGateway, err.Error())
            return
        }

        // 3. Write response
        w.WriteHeader(out.Status)
        w.Write(out.Body)
    }
}
```

**Правила:**
- Handler НЕ содержит бизнес-логику
- Handler НЕ обращается к БД напрямую
- Handler использует интерфейсы для зависимостей
- Handler логирует входящие запросы и исходящие ответы

### 3.2 Policy Layer (`internal/domain/<name>/policy/`)

**Ответственность:**
- Оркестрация use-cases (сценариев использования)
- Валидация доменных инвариантов
- Координация между Service и Upstream
- Обработка callbacks

**Паттерн Policy:**

```go
type Policy struct {
    Svc service.Service    // Service layer
    Up  Upstream           // External API interface
    // Optional dependencies for callbacks
    PartnerURLs interface {
        GetCallbackURL(ctx context.Context, partnerID string, operation string) (string, error)
    }
}

func New(svc service.Service, up Upstream) *Policy {
    return &Policy{Svc: svc, Up: up}
}

// Use-case method
func (p Policy) Attach(ctx context.Context, in AttachInput) (AttachOutput, error) {
    // 1. Validate domain invariants
    if in.PartnerID == "" {
        return AttachOutput{}, ErrEmptyPartner
    }

    // 2. Call upstream
    upResp, err := p.Up.Attach(ctx, AttachUpstreamRequest{...})
    if err != nil {
        return AttachOutput{}, err
    }

    // 3. Persist via service
    if upResp.Status >= 200 && upResp.Status < 300 {
        _ = p.Svc.CreateApplication(ctx, in.PartnerID, "attach", abrID, "accepted", &detachDate)
    }

    return AttachOutput{Status: upResp.Status, Body: upResp.Body}, nil
}
```

**Правила:**
- Policy НЕ обращается к БД напрямую (только через Service)
- Policy определяет интерфейсы для внешних зависимостей
- Policy координирует несколько сервисов при необходимости

### 3.3 Service Layer (`internal/domain/<name>/service/`)

**Ответственность:**
- Бизнес-логика и бизнес-правила
- Координация нескольких DAO
- Управление транзакциями
- Генерация отчетов

**Паттерн Service:**

```go
type Service struct {
    Applications dao.Applications
    IMEIs        dao.IMEIOperations
    AppState     ApplicationStateStore  // Interface for state storage
}

func New(apps dao.Applications, imeis dao.IMEIOperations, state ApplicationStateStore) Service {
    return Service{Applications: apps, IMEIs: imeis, AppState: state}
}

func (s Service) CreateApplication(ctx context.Context, partnerID, kind, abrID, status string, detachTermAt *time.Time) error {
    if err := s.Applications.InsertApplication(ctx, partnerID, kind, abrID, status, detachTermAt); err != nil {
        return err
    }
    return s.setAppState(ctx, abrID, status)
}
```

**Правила:**
- Service содержит бизнес-логику
- Service не знает о HTTP (нет импорта `net/http`)
- Service возвращает доменные ошибки, не HTTP статусы
- Service может координировать несколько DAO

### 3.4 DAO Layer (`internal/domain/<name>/dao/`)

**Ответственность:**
- Чистый доступ к данным
- SQL запросы
- Mapping между DB и domain models
- Transaction management

**Паттерн DAO:**

```go
type Applications struct {
    DB postgresql.Client
}

func (d Applications) InsertApplication(ctx context.Context, partnerID, kind, abrID, status string, detachTermAt *time.Time) error {
    const q = `INSERT INTO applications (partner_id, kind, abr_id, status, detach_term_at)
               VALUES ($1, $2::application_kind, $3, $4::application_status, $5)`
    _, err := d.DB.Exec(ctx, q, partnerID, kind, abrID, status, detachTermAt)
    return err
}

func (d Applications) GetByABRID(ctx context.Context, abrID string) (*Application, error) {
    const q = `SELECT id, partner_id, kind, abr_id, status, detach_term_at FROM applications WHERE abr_id = $1`
    row := d.DB.QueryRow(ctx, q, abrID)
    var a Application
    if err := row.Scan(&a.ID, &a.PartnerID, &a.Kind, &a.ABRID, &a.Status, &a.DetachTermAt); err != nil {
        return nil, err
    }
    return &a, nil
}
```

**Правила:**
- DAO НЕ содержит бизнес-логику
- DAO возвращает domain models, не raw DB types
- DAO использует prepared statements или sqlc
- Сложные операции выносятся в PL/pgSQL функции

---

## 4. Composition Root

### 4.1 Структура App

`internal/app/app.go` - центральная точка сборки приложения:

```go
type App struct {
    cfg        config.Config
    httpServer *http.Server
    rdb        *redis.Client
    router     *chi.Mux
    pg         postgresql.Client

    // Domain interfaces (not concrete types)
    subcontrolPolicy interface {
        Attach(ctx context.Context, in AttachInput) (AttachOutput, error)
        // ...
    }
    secretProvider interface {
        GetInboundSecret(ctx context.Context, projectID string) ([]byte, error)
    }
}
```

### 4.2 Инициализация

```go
func NewApp(ctx context.Context, cfg config.Config) (*App, error) {
    // 1. Infrastructure layer
    rdb := database.NewRedisClient(cfg.Redis.Addr)
    pgclient, err := database.NewPostgresClient(ctx, cfg.Postgres.DSN)
    if err != nil {
        return nil, err
    }

    // 2. Router with middleware
    r := chi.NewRouter()
    r.Use(chimid.RequestID)
    r.Use(chimid.RealIP)
    r.Use(chimid.Recoverer)
    r.Use(mw.RequestLogger())

    // 3. Initialize App struct
    a := &App{cfg: cfg, rdb: rdb, router: r, pg: pgclient}

    // 4. DAOs
    partnersDAO := dao.Partners{DB: pgclient}
    appsDAO := dao.Applications{DB: pgclient}
    imeisDAO := &dao.IMEIsSqlc{DB: pgclient}

    // 5. Secret provider with caching
    a.secretProvider = secretcache.NewWithRedis(rdb, partnersDAO, 5*time.Minute)

    // 6. External clients
    upClient := scon.New(cfg.Upstream.BaseURL, cfg.Upstream.Username, cfg.Upstream.Password)

    // 7. Services
    appState := redisx.AppState{R: rdb, TTL: 10 * time.Minute}
    scService := service.New(appsDAO, imeisDAO, appState)

    // 8. Policy
    a.subcontrolPolicy = policy.New(scService, upClient)

    // 9. Register routes
    a.registerRoutes()

    return a, nil
}
```

### 4.3 Route Registration

```go
func (a *App) registerRoutes() {
    // Public
    a.router.Get("/healthz", a.healthHandler)

    // HMAC-protected routes
    verifyCfg := mw.VerifyConfig{
        Skew:     time.Duration(a.cfg.Security.SkewSec) * time.Second,
        NonceTTL: time.Duration(a.cfg.Security.NonceTTLSec) * time.Second,
    }
    nonces := redisx.Nonces{R: a.rdb}
    subH := httpcontroller.NewSubcontrolHandler(a.subcontrolPolicy)

    a.router.Group(func(pr chi.Router) {
        pr.Use(mw.HMACVerify(verifyCfg, nonces, a.secretProvider.GetInboundSecret))
        pr.Post("/api/v1/attach", subH.Attach())
        pr.Post("/api/v1/detach", subH.Detach())
    })

    // JWT-protected admin routes
    a.router.Group(func(pr chi.Router) {
        pr.Use(mw.RequireSuperAdmin(jwtSecretBytes))
        pr.Get("/admin/partners", adminH.ListPartners)
    })
}
```

---

## 5. Domain Layer

### 5.1 Организация домена

Каждый bounded context (домен) имеет свою директорию:

```
internal/domain/<domain_name>/
├── policy/           # Use-case orchestration
│   └── policy.go
├── service/          # Business logic
│   └── service.go
└── dao/              # Data access
    ├── <entity>.go       # Manual SQL
    └── <entity>_sqlc.go  # sqlc-generated
```

### 5.2 Определение интерфейсов

**Правило:** Интерфейс определяется там, где он используется, не там где реализуется.

```go
// В policy.go - Policy определяет интерфейс Upstream
type Upstream interface {
    Attach(ctx context.Context, req AttachUpstreamRequest) (AttachUpstreamResponse, error)
    Detach(ctx context.Context, imeis []string) (AttachUpstreamResponse, error)
}

// В service.go - Service определяет интерфейс для state storage
type ApplicationStateStore interface {
    Set(ctx context.Context, abrID string, status string, success, failed int, reportID *string) error
}
```

### 5.3 Input/Output Types

Каждый use-case имеет явные типы для входа и выхода:

```go
// Input types
type AttachInput struct {
    PartnerID       string
    DetachTermMonth int
    Devices         [][]string
}

// Output types
type AttachOutput struct {
    Status  int
    Headers map[string][]string
    Body    []byte
    ABRID   string
}
```

---

## 6. Transport Layer

### 6.1 Middleware Pattern

```go
func HMACVerify(cfg VerifyConfig, nonces NonceStore, secretProvider func(ctx context.Context, projectID string) ([]byte, error)) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // 1. Extract headers
            tsStr := r.Header.Get("X-Timestamp")
            nonce := r.Header.Get("X-Nonce")
            pid := r.Header.Get("X-Project-Id")
            sigHdr := r.Header.Get("X-Signature")

            // 2. Validate
            if tsStr == "" || nonce == "" || pid == "" || sigHdr == "" {
                http.Error(w, "missing signature headers", http.StatusUnauthorized)
                return
            }

            // 3. Check timestamp skew
            ts, err := time.Parse(time.RFC3339, tsStr)
            if err != nil || time.Since(ts) > cfg.Skew {
                http.Error(w, "timestamp skew", http.StatusUnauthorized)
                return
            }

            // 4. Anti-replay check
            if ok, _ := nonces.TryReserve(nonce, cfg.NonceTTL); !ok {
                http.Error(w, "replay detected", http.StatusForbidden)
                return
            }

            // 5. Verify signature
            secret, err := secretProvider(r.Context(), pid)
            if err != nil {
                http.Error(w, "project forbidden", http.StatusForbidden)
                return
            }
            // ... verify HMAC ...

            // 6. Inject to context
            ctx := context.WithValue(r.Context(), CtxProjectIDKey, pid)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}
```

### 6.2 Response Helpers

```go
// internal/httpx/response/response.go
func Error(w http.ResponseWriter, code int, message string) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(code)
    json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func JSON(w http.ResponseWriter, code int, data any) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(code)
    json.NewEncoder(w).Encode(data)
}
```

---

## 7. Infrastructure

### 7.1 Database Client

```go
// internal/database/postgres.go
func NewPostgresClient(ctx context.Context, dsn string) (postgresql.Client, error) {
    poolConfig, err := pgxpool.ParseConfig(dsn)
    if err != nil {
        return nil, err
    }
    poolConfig.MaxConns = 25
    poolConfig.MinConns = 5

    pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
    if err != nil {
        return nil, err
    }
    return pool, nil
}
```

### 7.2 Configuration

```go
// internal/config/config.go
type Config struct {
    Server   Server   `yaml:"server"`
    Security Security `yaml:"security"`
    Postgres Postgres `yaml:"postgres"`
    Redis    Redis    `yaml:"redis"`
}

type Postgres struct {
    DSN string `yaml:"dsn" env:"DB_URL,required"`
}

func MustLoad() Config {
    _ = godotenv.Load()  // Load .env if exists

    var cfg Config
    if err := cleanenv.ReadEnv(&cfg); err != nil {
        log.Fatalf("config env error: %v", err)
    }
    return cfg
}
```

### 7.3 Redis Stores

```go
// internal/store/redisx/nonce.go
type Nonces struct {
    R *redis.Client
}

func (n Nonces) TryReserve(nonce string, ttl time.Duration) (bool, error) {
    ok, err := n.R.SetNX(context.Background(), "nonce:"+nonce, "1", ttl).Result()
    return ok, err
}
```

---

## 8. Паттерны и конвенции

### 8.1 Именование

| Элемент | Конвенция | Пример |
|---------|-----------|--------|
| Package | lowercase, singular | `dao`, `service`, `policy` |
| Interface | Описывает поведение | `Upstream`, `NonceStore` |
| Struct | CamelCase, noun | `SubcontrolHandler`, `Applications` |
| Constructor | `New<Type>` | `NewSubcontrolHandler()` |
| Handler methods | Verb, returns `http.HandlerFunc` | `Attach() http.HandlerFunc` |
| DAO methods | CRUD verbs | `Insert`, `Get`, `Update`, `Delete`, `List` |

### 8.2 Error Handling

```go
// Define domain errors
var (
    ErrPartnerInactive = errors.New("partner inactive or not found")
    ErrEmptyPartner    = errors.New("empty partner id")
)

// Return domain errors, not HTTP codes
func (p Policy) Attach(ctx context.Context, in AttachInput) (AttachOutput, error) {
    if in.PartnerID == "" {
        return AttachOutput{}, ErrEmptyPartner
    }
    // ...
}
```

### 8.3 Context Usage

- Context передается первым аргументом во все методы
- Логгер извлекается из context: `logging.L(ctx)`
- Данные аутентификации хранятся в context

```go
func (h *Handler) SomeEndpoint(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    partnerID := ctx.Value(CtxProjectIDKey).(string)
    logging.L(ctx).Info("Processing request", logging.StringAttr("partner_id", partnerID))
}
```

### 8.4 Transactions

```go
func (d *DAO) ComplexOperation(ctx context.Context, data Data) error {
    tx, err := d.DB.Begin(ctx)
    if err != nil {
        return err
    }
    defer tx.Rollback(ctx)  // Rollback if not committed

    // Multiple operations...
    if err := step1(ctx, tx); err != nil {
        return err
    }
    if err := step2(ctx, tx); err != nil {
        return err
    }

    return tx.Commit(ctx)  // Commit overrides rollback
}
```

### 8.5 Parallel Processing

```go
func (p Policy) OnCallback(ctx context.Context, cb Callback) error {
    var wg sync.WaitGroup
    var mu sync.Mutex
    var firstErr error

    for _, group := range cb.Groups {
        group := group  // Capture loop variable
        wg.Add(1)

        go func() {
            defer wg.Done()

            if err := p.processGroup(ctx, group); err != nil {
                mu.Lock()
                if firstErr == nil {
                    firstErr = err
                }
                mu.Unlock()
            }
        }()
    }

    wg.Wait()
    return firstErr
}
```

---

## 9. Быстрый старт нового проекта

### 9.1 Создание структуры директорий

```bash
#!/bin/bash
PROJECT_NAME=$1

mkdir -p $PROJECT_NAME/{cmd/api,migrations,docs,scripts}
mkdir -p $PROJECT_NAME/internal/{app,config}
mkdir -p $PROJECT_NAME/internal/controller/http
mkdir -p $PROJECT_NAME/internal/domain
mkdir -p $PROJECT_NAME/internal/database/sqlc
mkdir -p $PROJECT_NAME/internal/httpx/{middleware,response,upstream}
mkdir -p $PROJECT_NAME/internal/store/redisx
mkdir -p $PROJECT_NAME/internal/logx
mkdir -p $PROJECT_NAME/pkg/postgresql

# Create initial files
touch $PROJECT_NAME/cmd/api/main.go
touch $PROJECT_NAME/internal/app/app.go
touch $PROJECT_NAME/internal/config/config.go
touch $PROJECT_NAME/Makefile
touch $PROJECT_NAME/docker-compose.yml
touch $PROJECT_NAME/Dockerfile
touch $PROJECT_NAME/sqlc.yaml
touch $PROJECT_NAME/.env.example
touch $PROJECT_NAME/go.mod
```

### 9.2 Добавление нового домена

```bash
DOMAIN_NAME=$1

mkdir -p internal/domain/$DOMAIN_NAME/{policy,service,dao}
touch internal/domain/$DOMAIN_NAME/policy/policy.go
touch internal/domain/$DOMAIN_NAME/service/service.go
touch internal/domain/$DOMAIN_NAME/dao/${DOMAIN_NAME}.go
```

### 9.3 Минимальный main.go

```go
package main

import (
    "context"
    "log"
    "os"

    "github.com/theartofdevel/logging"

    "yourproject/internal/app"
    "yourproject/internal/config"
)

func main() {
    cfg := config.MustLoad()

    ctx := logging.ContextWithLogger(context.Background(), logging.NewLogger(
        logging.WithLevel(cfg.Logging.Level),
    ))

    application, err := app.NewApp(ctx, cfg)
    if err != nil {
        log.Fatalf("app init error: %v", err)
    }

    if err := application.Run(ctx); err != nil {
        log.Fatalf("app run error: %v", err)
    }
}
```

### 9.4 Чек-лист для нового домена

- [ ] Создать структуру директорий: `domain/<name>/{policy,service,dao}`
- [ ] Определить domain models в `dao/`
- [ ] Реализовать DAO с SQL операциями
- [ ] Реализовать Service с бизнес-логикой
- [ ] Реализовать Policy для оркестрации (если нужен)
- [ ] Создать HTTP handler в `controller/http/`
- [ ] Зарегистрировать в composition root (`app.go`)
- [ ] Добавить миграции в `migrations/`
- [ ] Написать E2E тесты

---

## Appendix A: Полезные Make-команды

```makefile
# Development
dev:
    go run ./cmd/api

# Testing
test:
    go test ./...

test-e2e:
    go test -v ./internal/integration/...

# Database
migrate-up:
    goose -dir migrations postgres "$(DB_URL)" up

migrate-down:
    goose -dir migrations postgres "$(DB_URL)" down

migrate-create:
    goose -dir migrations create $(NAME) sql

# Docker
docker-build:
    docker build -t $(PROJECT):latest .

docker-up:
    docker-compose up -d

# SQLC
sqlc-generate:
    sqlc generate
```

---

## Appendix B: Зависимости (go.mod)

```
github.com/go-chi/chi/v5           // HTTP router
github.com/jackc/pgx/v5            // Postgres driver
github.com/redis/go-redis/v9       // Redis client
github.com/ilyakaznacheev/cleanenv // Config from ENV
github.com/joho/godotenv           // .env file support
github.com/theartofdevel/logging   // Structured logging (slog wrapper)
github.com/golang-jwt/jwt/v5       // JWT tokens
github.com/pressly/goose/v3        // Migrations
```

---

**Документ создан на основе анализа проекта able-sc**
