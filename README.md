# portal-plugin-quota

A quota management plugin for the portal framework that provides flexible quota enforcement with multiple policies, usage tracking, and comprehensive API extensions for managing storage, upload, and download quotas.

## Features

- **Multiple Enforcement Policies**
  - **HARD_LIMITS**: Strict limits blocking operations when exceeded
  - **ALLOWANCE**: Pre-paid allowance consumption with grant tracking
  - **THRESHOLD**: Soft limits with warnings, allows exceeding threshold
  - **UNLIMITED**: No limits, only records usage

- **Usage Tracking**
  - Records upload/download/storage operations
  - Tracks daily and detailed usage
  - Provides usage history queries

- **Plan Management**
  - Reusable quota templates with limits and thresholds
  - Per-user configuration with plan assignment
  - Custom limits override support

- **API Extensions**
  - Admin API for plan and allowance management
  - User-facing API for usage queries
  - Swagger/OpenAPI documentation

- **Observability**
  - Prometheus metrics for policy checks and operations
  - OpenTelemetry tracing throughout
  - Comprehensive logging

## Installation

### Prerequisites

- Go 1.26.0 or later
- Portal framework

### Build

```bash
# Build the plugin
go build -o quota.so

# Build with build metadata
go build -ldflags "-X main.Version=1.0.0 -X main.GitCommit=$(git rev-parse HEAD)"
```

### Install

Copy the built plugin to your portal plugins directory:

```bash
cp quota.so /path/to/portal/plugins/
```

## Configuration

The plugin is configured through the portal configuration system with the following options:

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `enabled` | bool | `true` | Controls whether the quota service is active |
| `default_enforcement_policy` | string | `HARD_LIMITS` | Default policy for new users (HARD_LIMITS, UNLIMITED, ALLOWANCE, THRESHOLD) |
| `reconciliation_hour` | int | `2` | Hour (0-23) when daily quota reconciliation runs |
| `history_retention_days` | int | `90` | How long to retain quota history records |
| `detailed_retention_days` | int | `30` | How long to retain detailed usage records |
| `enable_shared_usage` | bool | `true` | Enables shared usage tracking |
| `shared_usage_precision` | int | `2` | Decimal precision for shared usage (0-10) |
| `default_quota_plan_name` | string | `default` | Quota plan assigned to new users |

### Example Configuration

```yaml
quota:
  enabled: true
  default_enforcement_policy: "HARD_LIMITS"
  reconciliation_hour: 2
  history_retention_days: 90
  detailed_retention_days: 30
  enable_shared_usage: true
  shared_usage_precision: 2
  default_quota_plan_name: "default"
```

## API

### Admin API

The plugin provides an admin API extension at `/api/quota/` for managing plans and allowances:

- **Plan Management**
  - `GET /api/quota/plans` - List all quota plans
  - `POST /api/quota/plans` - Create a new quota plan
  - `GET /api/quota/plans/:id` - Get a specific plan
  - `PUT /api/quota/plans/:id` - Update a plan
  - `DELETE /api/quota/plans/:id` - Delete a plan
  - `POST /api/quota/plans/:id/default` - Set as default plan

- **Allowance Management**
  - `GET /api/quota/allowances` - List allowances
  - `POST /api/quota/allowances` - Create an allowance grant
  - `PUT /api/quota/allowances/:id` - Update an allowance
  - `DELETE /api/quota/allowances/:id` - Deactivate an allowance

- **System Management**
  - `GET /api/quota/system/stats` - Get system statistics
  - `POST /api/quota/system/reconcile` - Trigger quota reconciliation
  - `POST /api/quota/system/cleanup` - Clean up old usage records
  - `GET /api/quota/system/config` - Get current configuration

### User API

Authenticated users can query their quota status and usage history:

- `GET /api/account/quota` - Get current quota status
- `GET /api/account/quota/history` - Get quota usage history for charting and analytics

## Project Structure

```
portal-plugin-quota/
├── quota.go                    # Plugin registration
├── core/                       # Public interfaces and types
│   ├── quota_service.go
│   ├── usage_manager.go
│   ├── grant_manager.go
│   └── ...
├── internal/
│   ├── service/
│   │   ├── quota/              # Main quota service
│   │   ├── managers/           # Specialized managers
│   │   └── policies/           # Policy enforcement implementations
│   ├── api/                    # HTTP endpoints
│   ├── db/
│   │   ├── models/             # Database models
│   │   └── migrations/         # Database migrations
│   └── config/                 # Configuration schema
└── build/                      # Build metadata
```

## Development

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

# Run tests with coverage
go test ./... -cover

# Run integration tests (require database)
go test ./internal/service/policies/hard_limits_integration_test.go -v
```

### Code Generation

```bash
# Install mockery for mock generation
go install github.com/vektra/mockery/v3@v3.7.0

# Generate mocks
$HOME/go/bin/mockery
```

### Linting

```bash
# Run go vet
go vet ./...

# Run golangci-lint
golangci-lint run
```

## Architecture

The plugin follows a layered architecture:

1. **Core Layer** (`core/`): Defines public interfaces and types
2. **Service Layer** (`internal/service/`): Implements business logic
   - `quota/`: Main quota service implementation
   - `managers/`: Specialized managers (usage, grants, config)
   - `policies/`: Policy enforcement strategies
3. **API Layer** (`internal/api/`): HTTP handlers and extensions
4. **Database Layer** (`internal/db/`): GORM models and migrations

### Policy Enforcement Flow

1. Quota check requested via `QuotaService.CheckXQuota()`
2. Get user config via `ConfigManager.GetUserQuotaConfig()`
3. Get appropriate policy enforcer via `ConfigManager.GetPolicyEnforcer()`
4. Enforcer checks limits using `LimitResolver.ResolvedEffectiveLimits()`
5. Result includes allowed/disallowed status, reason, and details

### Database Models

Key models include:

- `QuotaPlan`: Reusable quota templates
- `UserQuotaConfig`: Per-user configuration
- `AllowanceGrant`: Pre-paid allowances
- `AllowanceConsumption`: Consumption tracking
- `UserUsageDetail`: Usage records by type
- `QuotaCheckReason`: Check result enumeration
- `EnforcementPolicy`: Policy enumeration
- `UsageType`: Usage type enumeration
- `GrantType`: Grant type enumeration

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Support

For issues and questions, please refer to the portal framework documentation.
