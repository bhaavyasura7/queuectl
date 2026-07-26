package types

type Job struct {
	ID            string
	Command       string
	State         string
	Priority      string
	RunAt         *string
	Timeout       *int
	Attempts      int
	MaxRetries    int
	BackoffBase   float64
	CreatedAt     string
	UpdatedAt     string
	NextAttemptAt *string
	ClaimedAt     *string
	HeartbeatAt   *string
	WorkerID      *string
	LastError     *string
	Stdout        *string
	Stderr        *string
}

type WorkerRow struct {
	PID       int
	Hostname  string
	StartedAt string
	Status    string
}

type ConfigRow struct {
	Key   string
	Value string
}
