package quota

import (
	pluginCore "go.lumeweb.com/portal-plugin-quota/core"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	MetricUploadChecked    = "upload_quota_checked_total"
	MetricDownloadChecked  = "download_quota_checked_total"
	MetricStorageChecked   = "storage_quota_checked_total"
	MetricUploadRecorded   = "upload_recorded_total"
	MetricDownloadRecorded = "download_recorded_total"
	MetricStorageRecorded  = "storage_recorded_total"
	MetricAllowanceAdded   = "allowance_added_total"
	MetricAllowanceBalance = "allowance_balance_bytes"
	MetricDuration         = "operation_duration_seconds"
)

const (
	LabelStatusAllowed   = "allowed"
	LabelStatusDenied    = "denied"
	LabelStatusError     = "error"
	LabelOperationCheck  = "check"
	LabelOperationRecord = "record"
	LabelTypeStorage     = "storage"
	LabelTypeUpload      = "upload"
	LabelTypeDownload    = "download"
)

var (
	UploadChecked     *prometheus.CounterVec
	DownloadChecked   *prometheus.CounterVec
	StorageChecked    *prometheus.CounterVec
	UploadRecorded    *prometheus.CounterVec
	DownloadRecorded  *prometheus.CounterVec
	StorageRecorded   *prometheus.CounterVec
	AllowanceAdded    *prometheus.CounterVec
	AllowanceBalance  *prometheus.GaugeVec
	OperationDuration *prometheus.HistogramVec
)

func init() {
	UploadChecked = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:      MetricUploadChecked,
			Subsystem: pluginCore.QUOTA_SERVICE,
			Help:      "Total number of upload quota checks",
		},
		[]string{"status"},
	)

	DownloadChecked = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:      MetricDownloadChecked,
			Subsystem: pluginCore.QUOTA_SERVICE,
			Help:      "Total number of download quota checks",
		},
		[]string{"status"},
	)

	StorageChecked = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:      MetricStorageChecked,
			Subsystem: pluginCore.QUOTA_SERVICE,
			Help:      "Total number of storage quota checks",
		},
		[]string{"status"},
	)

	UploadRecorded = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:      MetricUploadRecorded,
			Subsystem: pluginCore.QUOTA_SERVICE,
			Help:      "Total number of upload recordings",
		},
		[]string{"status"},
	)

	DownloadRecorded = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:      MetricDownloadRecorded,
			Subsystem: pluginCore.QUOTA_SERVICE,
			Help:      "Total number of download recordings",
		},
		[]string{"status"},
	)

	StorageRecorded = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:      MetricStorageRecorded,
			Subsystem: pluginCore.QUOTA_SERVICE,
			Help:      "Total number of storage recordings",
		},
		[]string{"status"},
	)

	AllowanceAdded = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:      MetricAllowanceAdded,
			Subsystem: pluginCore.QUOTA_SERVICE,
			Help:      "Total number of allowance additions by source",
		},
		[]string{"source"},
	)

	AllowanceBalance = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name:      MetricAllowanceBalance,
			Subsystem: pluginCore.QUOTA_SERVICE,
			Help:      "Current allowance balance in bytes by type",
		},
		[]string{"type"},
	)

	OperationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:      MetricDuration,
			Subsystem: pluginCore.QUOTA_SERVICE,
			Help:      "Duration of quota operations",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"operation"},
	)
}

func GetCollectors() []prometheus.Collector {
	return []prometheus.Collector{
		UploadChecked,
		DownloadChecked,
		StorageChecked,
		UploadRecorded,
		DownloadRecorded,
		StorageRecorded,
		AllowanceAdded,
		AllowanceBalance,
		OperationDuration,
	}
}
