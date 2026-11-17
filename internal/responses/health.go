package responses

// HealthResponse is returned by GET /health.
type HealthResponse struct {
	Status   string `json:"status"`
	Database string `json:"database"`
}
