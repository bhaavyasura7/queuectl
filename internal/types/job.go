package types

type Job struct {
	ID            string  `json:"id"`
	Command       string  `json:"command"`
	State         string  `json:"state"`
	Priority      string  `json:"priority"`
	RunAt         *string `json:"run_at,omitempty"`
	Timeout       *int    `json:"timeout,omitempty"`
	Attempts      int     `json:"attempts"`
	MaxRetries    int     `json:"max_retries"`
	BackoffBase   float64 `json:"backoff_base"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`
	NextAttemptAt *string `json:"next_attempt_at,omitempty"`
	ClaimedAt     *string `json:"claimed_at,omitempty"`
	HeartbeatAt   *string `json:"heartbeat_at,omitempty"`
	WorkerID      *string `json:"worker_id,omitempty"`
	LastError     *string `json:"last_error,omitempty"`
	Stdout        *string `json:"stdout,omitempty"`
	Stderr        *string `json:"stderr,omitempty"`
}

type WorkerRow struct {
	PID       int
	Hostname  string
	StartedAt string
	Status    string
}


