## 0.1.0 (2026-05-27)

### Breaking Changes

- Replaces fixed daily/lifetime limits with flexible window-based quotas. Database schema changes require migration. Window configuration system supports ROLLING, DAY, WEEK, MONTH, YEAR, and LIFETIME time windows. Administration, policy enforcement, and querying layers updated for window-aware quota management.

- Add WindowType enum: ROLLING, DAY, WEEK, MONTH, YEAR, LIFETIME
- Add LimitWindow struct with configurable time boundaries
- Update QuotaPlan and UserQuotaConfig with window configuration fields
- Implement window-aware quota enforcement in all policy enforcers
- Add window-based usage queries across all time window types
- Update admin API to support window configuration
- Add database migrations for window configuration fields
- Refactor limit resolver to support windowed limits
- Update tests across quota, policy, and API layers
- user API now displays reserved and committed quota separately
- Initial version

### Features

- initialize quota management plugin with database models and migrations
- implement quota policy enforcers for allowance, hard limits, threshold, and unlimited
- implement centralized usage manager and shared usage calculation
- implement grant management and atomic usage updates
- implement ConfigManager and add related core interfaces
- implement quota service with allowance management
- add RemoveUserFromPlan functionality
- implement event listeners and update download tracking logic
- add comprehensive quota admin API with system stats
- add dashboard API extension for quota operations
- add group quota availability checking for CID operations
- add admin APIs for user quota config management
- add lock manager to prevent quota race conditions
- add reservation system to prevent thundering herd
- add reservation debug logging with concurrent request count
- add quota plan changed event emission
- add redundancy-based storage scaling

### Fixes

- remove unique constraint from user_id in user_quota_configs table and update validation logic
- validate IP address format and shared_with range in user usage details
- remove redundant SharedWith negative validation check
- correct allowance grant underflow comment and add timestamp auto-setting
- allow ReconciliationHour to include midnight (0) in validation range
- correct quota policy enforcer validation order and storage calculation
- improve error handling and usage calculations in quota policies
- correct usage aggregation and default plan handling
- add default unlimited quota config in usage recording tests
- update updated_at timestamp when deactivating allowance grants>
- update updated_at timestamp when consuming bytes from grants
- update GetActiveGrantsLocked interface signature to include tx parameter
- ensure time comparisons use UTC in grant retrieval and creation
- use locked grants when consuming quota allowances
- validate quota plan existence before user assignment
- address critical issues from code review
- add thread safety to system configuration access
- address PR review feedback for quota API extension
- remove StatementBegin/StatementEnd from MySQL migration
- add WithPathParam definitions for routes with path parameters
- add swagger response schemas to quota endpoints
- add defensive programming for duplicate quota plan names
- handle database errors and TOCTOU race condition in CreateQuotaPlan
- handle ErrQuotaPlanNotFound in duplicate name check
- conditionally skip validation only when name is unchanged
- validate limits during partial updates
- propagate errors from First/Save and trigger BeforeSave hook
- resolve nil pointer dereference in UsageAggregator initialization
- resolve duplicate entry error when switching default quota plan
- prevent UpdateQuotaPlan from changing default plan
- prevent CreateQuotaPlan from setting IsDefault=true
- handle soft-deleted plans when setting default quota plan
- correct swagger response field mappings for list endpoints
- handle all enforcement policies in quota status API
- address PR review feedback - context, storage, threshold
- resolve float64 precision loss in distribution algorithm
- use request context for CID resolution instead of service background context
- address PR review feedback for admin APIs
- add missing swagger success response to /plans/:planID/default endpoint
- remove default 200 ErrorResponse from DELETE/204 endpoints
- address PR review feedback
- resolve Zog type cast error in UserQuotaConfigUpdateRequest
- resolve memory leaks and goroutine leaks in lock manager
- add STORAGE_ADD mock expectations and make lock fields atomic
- address PR #104 review feedback
- address PR review feedback for reservation system
- ensure concurrent reservation test runs safely without panics or data races
- ensure consistent lock acquisition for anonymous operations
- correct window_usage_before logging to prevent integer underflow
- remove incorrect subtraction of reserved bytes from used values
- enforce authentication middleware on admin routes
- use target API subdomain for admin route registration
- use Attrs() instead of init struct in FirstOrCreate to prevent duplicate key violations
