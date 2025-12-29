package policies

import (
	pluginCore "go.lumeweb.com/portal-plugin-quota/core"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	MetricPolicyChecks   = "policy_checks_total"
	MetricPolicyDuration = "policy_check_duration_seconds"
	MetricLimitResolved  = "limits_resolved_total"
	MetricPlanOperations = "plan_operations_total"
)

const (
	LabelPolicyHardLimits = "hard_limits"
	LabelPolicyThreshold  = "threshold"
	LabelPolicyUnlimited  = "unlimited"
	LabelPolicyAllowance  = "allowance"
)

const (
	LabelResultAllowed = "allowed"
	LabelResultDenied  = "denied"
	LabelResultError   = "error"
	LabelResultWarning = "warning"
)

const (
	LabelLimitSourcePlan = "plan"
	LabelLimitSourceUser = "user"
)

const (
	LabelPlanOperationCreate = "create"
	LabelPlanOperationUpdate = "update"
	LabelPlanOperationDelete = "delete"
	LabelPlanOperationGet    = "get"
)

var (
	PolicyChecks      *prometheus.CounterVec
	PolicyDuration    *prometheus.HistogramVec
	LimitResolved     *prometheus.CounterVec
	PlanOperations    *prometheus.CounterVec
	PlanOperationsErr *prometheus.CounterVec
)

func init() {
	PolicyChecks = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:      MetricPolicyChecks,
			Subsystem: pluginCore.QUOTA_SERVICE,
			Help:      "Total number of policy checks by policy type and result",
		},
		[]string{"policy", "result"},
	)

	PolicyDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:      MetricPolicyDuration,
			Subsystem: pluginCore.QUOTA_SERVICE,
			Help:      "Duration of policy checks by policy type",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"policy"},
	)

	LimitResolved = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:      MetricLimitResolved,
			Subsystem: pluginCore.QUOTA_SERVICE,
			Help:      "Total number of limit resolutions by source",
		},
		[]string{"source"},
	)

	PlanOperations = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:      MetricPlanOperations,
			Subsystem: pluginCore.QUOTA_SERVICE,
			Help:      "Total number of quota plan operations by operation type",
		},
		[]string{"operation"},
	)

	PlanOperationsErr = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:      "plan_operations_errors_total",
			Subsystem: pluginCore.QUOTA_SERVICE,
			Help:      "Total number of quota plan operation errors by operation type",
		},
		[]string{"operation"},
	)
}

func GetCollectors() []prometheus.Collector {
	return []prometheus.Collector{
		PolicyChecks,
		PolicyDuration,
		LimitResolved,
		PlanOperations,
		PlanOperationsErr,
	}
}
