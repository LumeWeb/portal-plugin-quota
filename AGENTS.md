# AGENTS.md
This file provides guidance to various AI agents when working with code in this repository.

## Project Overview

This is a Go-based quota management plugin (`portal-plugin-quota`) for the portal framework. It provides flexible quota enforcement with multiple policies, usage tracking, and comprehensive API extensions for managing storage, upload, and download quotas.

## Common Commands

### Testing
```bash
# Run all tests
go test ./...

# Run tests for a specific package
go test ./internal/service/quota

# Run tests with verbose output
go test -v ./internal/service/policies

# Run a specific test
go test -run TestHardLimitsPolicyEnforcer_CheckUploadQuota_Success ./internal/service/policies
```

### Building
```bash
# Build the plugin
go build -o quota.so

# Build with build metadata
go build -ldflags "-X main.Version=1.0.0 -X main.GitCommit=$(git rev-parse HEAD)"
```

### Code Generation
```bash
# Install mockery
go install github.com/vektra/mockery/v3@v3.7.0

# Generate mocks
$HOME/go/bin/mockery

# The mockery configuration is in .mockery.yaml
```

### Linting and Quality
```bash
# Run go vet
go vet ./...

# Run golangci-lint (if installed)
golangci-lint run
```

### Database Migrations
Migrations are registered in `quota.go` and executed by the portal framework. Migration files are in:
- `internal/db/migrations/mysql/`
- `internal/db/migrations/sqlite/`

## High-Level Architecture

### Plugin Architecture
The project follows the portal framework's plugin architecture:

1. **Registration (`quota.go`)**: Plugin registers its services, API extensions, models, and migrations
2. **Core Layer (`core/`)**: Defines public interfaces and types that other plugins can use
3. **Service Layer (`internal/service/`)**: Implements business logic split into:
   - `quota/`: Main quota service implementation
   - `managers/`: Specialized managers for usage, grants, and config
   - `policies/`: Policy enforcement strategies
4. **API Layer (`internal/api/`)**: HTTP handlers and extensions for admin and user APIs
5. **Database Layer (`internal/db/`)**: GORM models and migrations

### Key Components

#### Core Interfaces (`core/`)
- `QuotaService`: Main service interface for quota operations
- `UsageManager`: Records and queries usage data
- `GrantManager`: Manages allowance grants (for ALLOWANCE policy)
- `ConfigManager`: Resolves user quota configuration
- `LimitResolver`: Resolves effective limits from plans and user overrides
- `QuotaPlanManager`: Manages quota plan database operations
- `UsageAggregator`: Aggregates usage statistics
- `PolicyEnforcer`: Interface for policy-specific enforcement logic

#### Policy Enforcement Patterns
The system supports **four enforcement policies**, each with a dedicated enforcer in `internal/service/policies/`:

1. **HARD_LIMITS** (`hard_limits.go`): Strict limits blocking operations when exceeded
2. **ALLOWANCE** (`allowance.go`): Pre-paid allowance consumption with grant tracking
3. **THRESHOLD** (`threshold.go`): Soft limits with warnings, allows exceeding threshold
4. **UNLIMITED** (`unlimited.go`): No limits, only records usage

All policy enforcers:
- Inherit from `BasePolicyEnforcer` for common validation and delegation
- Implement `PolicyEnforcer` interface
- Track metrics for policy checks
- Use `LimitResolver` to get effective limits

#### Service Managers (`internal/service/managers/`)

**UsageManager** (`usage.go`):
- Records upload/download/storage operations
- Tracks daily and detailed usage
- Provides usage history queries
- Uses GORM for database operations

**GrantManager** (`grant.go`):
- Creates and manages allowance grants
- Consumes bytes from grants with row-level locking
- Tracks grant types: storage, upload, download
- Manages grant lifecycle (create, deactivate, list)

**ConfigManager** (`config.go`):
- Resolves user quota configuration
- Returns appropriate policy enforcer
- Integrates LimitResolver, PlanManager, and policy enforcers

#### Database Models (`internal/db/models/`)
Key models include:
- `QuotaPlan`: Reusable quota templates with limits and thresholds
- `UserQuotaConfig`: Per-user configuration (policy, plan assignment, custom limits)
- `AllowanceGrant`: Pre-paid allowances for ALLOWANCE policy
- `AllowanceConsumption`: Tracks allowance consumption
- `UserUsageDetail`: Detailed usage records by type and timestamp
- `QuotaCheckReason`: Enumeration of check results (OK, LIMIT_EXCEEDED, ALLOWANCE_DEPLETED, etc.)
- `EnforcementPolicy`: Enumeration of enforcement policies
- `UsageType`: Enumeration of usage types (UPLOAD, DOWNLOAD, STORAGE_ADD, STORAGE_REMOVE)
- `GrantType`: Enumeration of grant types (STORAGE, UPLOAD, DOWNLOAD)

### API Extensions

Two main API extensions provide HTTP endpoints:

1. **AdminExtension** (`internal/api/admin_extension.go`): Admin-only endpoints at `/api/quota/`
   - Plan management: `/plans/*` (list, create, get, update, delete, set default)
   - Allowance management: `/allowances/*` (list, create, update, deactivate)
   - System management: `/system/*` (stats, reconcile, cleanup, config)

2. **QuotaExtension** (`internal/api/quota_extension.go`): User-facing endpoints
   - Usage queries and quota checks
   - Personal allowance management

Both use:
- `go.lumeweb.com/portal-router` for route registration
- Swagger/OpenAPI documentation builder
- Built-in authentication and authorization middleware

### Testing Framework

Test utilities are in `internal/testing/testing.go` and `testdata/manager.go`:

**TestDataManager**: Provides centralized test data management
- Atomic ID generation for users, uploads, plans, grants
- Resource tracking with cleanup
- Creates test entities with proper relationships

**Mock Setup**: Uses `testify/mock` with pre-generated mocks from mockery
- Mocks are in `core/mock_*.go`
- Generated from interfaces in `core/`
- Configure with `.EXPECT()` pattern

**Test Patterns**:
- Unit tests: Mock all dependencies
- Integration tests: Use real database with test data manager
- Use `internal/core/testing.RunTestCase` for framework integration

### Metrics and Observability

Prometheus metrics are defined in:
- `internal/service/policies/metrics.go`: Policy check metrics
- `internal/service/quota/metrics.go`: Operation metrics

Metrics tracked:
- Policy checks by type and result
- Operation duration histograms
- Plan operations by type
- Upload/download/storage checks and records
- Allowance balances

OpenTelemetry tracing is used throughout:
- `core.TraceMethod()` wraps methods for automatic tracing
- Spans are created for all service methods

### Configuration

Plugin configuration is in `internal/config/quota.go`:
- `Enabled`: Plugin enable/disable
- `DefaultEnforcementPolicy`: Default policy for new users
- `ReconciliationHour`: Daily reconciliation hour (0-23)
- `HistoryRetentionDays`: How long to keep history records
- `DetailedRetentionDays`: How long to keep detailed usage records
- `EnableSharedUsage`: Shared usage tracking toggle
- `SharedUsagePrecision`: Decimal precision (0-10)
- `DefaultQuotaPlanName`: Default plan name for new users

Uses `zog` schema validation with defaults.

## Important Patterns and Conventions

### Error Handling
- Model validation errors are predefined in `internal/db/models/errors.go`
- Use `fmt.Errorf` with `%w` for error wrapping
- Custom errors use descriptive `Err*` format

### Database Operations
- `db.RetryableComponentTransaction()` wrapper for retryable transactions
- GORM auto-migration handles schema updates
- Row-level locking with `GetActiveGrantsByTypeLocked()` for grant consumption

### Policy Resolution Flow
When a quota check is requested:
1. `QuotaService.CheckXQuota()` called
2. Gets user config via `ConfigManager.GetUserQuotaConfig()`
3. Gets policy enforcer via `ConfigManager.GetPolicyEnforcer()`
4. Calls enforcer's `CheckXQuota()` method
5. Enforcer may use `LimitResolver.ResolvedEffectiveLimits()`
6. Result includes allowed/disallowed status, reason, and details

### Limit Resolution
`LimitResolver` merges quotas from:
1. Quota plan (if assigned to user)
2. User-specific overrides
3. Default values when both are absent

Returns `EffectiveLimits` with tracking of which sources were configured.

### Dependency Management
- All managers are injected into `QuotaServiceDefault`
- Service implements `core.Service` and `core.Configurable` interfaces
- Uses `core.GetService[Type](ctx, serviceID)` for dependency resolution

## Testing Commands

```bash
# Run all tests
go test ./... -v

# Run specific test file
go test ./internal/service/policies/hard_limits_test.go -v

# Run tests with coverage
go test ./... -cover

# Run integration tests (require database)
go test ./internal/service/policies/hard_limits_integration_test.go -v
```

## Key File Locations

- `quota.go`: Plugin registration and metadata
- `core/`: Public interfaces and types
- `internal/service/quota/`: Main quota service
- `internal/service/managers/`: Specialized managers
- `internal/service/policies/`: Policy enforcement implementations
- `internal/api/`: HTTP endpoints
- `internal/db/models/`: Database models
- `internal/config/`: Configuration schema
- `.mockery.yaml`: Mock generation configuration
