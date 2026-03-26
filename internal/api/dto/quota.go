package dto

// QuotaStatusResponse represents current quota status for progress bars
type QuotaStatusResponse struct {
	Upload   QuotaTypeStatus `json:"upload"`
	Download QuotaTypeStatus `json:"download"`
	Storage  QuotaTypeStatus `json:"storage"`
}

// FromModel implements DTOResponse interface
func (r QuotaStatusResponse) FromModel(_ QuotaStatusResponse) error {
	return nil
}

// QuotaTypeStatus represents status for a single quota type
type QuotaTypeStatus struct {
	Used       uint64  `json:"used"`
	Limit      *uint64 `json:"limit,omitempty"`
	Remaining  *uint64 `json:"remaining,omitempty"`
	Percentage *int    `json:"percentage"`
	Threshold  *uint64 `json:"threshold,omitempty"`
}

// QuotaHistoryRequest parameters for historical data query
type QuotaHistoryRequest struct {
	StartDate string `query:"start_date"` // Required, RFC3339 format
	EndDate   string `query:"end_date"`   // Required, RFC3339 format
	Type      string `query:"type"`       // Optional: "upload" or "download"
}

// QuotaHistoryResponse represents historical usage data
type QuotaHistoryResponse struct {
	UserID uint         `json:"user_id"`
	Points []UsagePoint `json:"points"`
}

// FromModel implements DTOResponse interface
func (r QuotaHistoryResponse) FromModel(_ QuotaHistoryResponse) error {
	return nil
}

// UsagePoint represents a single data point in usage history
type UsagePoint struct {
	Date  string `json:"date"`
	Bytes uint64 `json:"bytes"`
}
