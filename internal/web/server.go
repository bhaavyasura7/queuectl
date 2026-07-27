package web

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"queuectl/internal/store"
	"queuectl/internal/types"
)

type Server struct {
	st *store.Store
}

func NewServer(st *store.Store) *Server {
	return &Server{st: st}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.dashboard)
	mux.HandleFunc("/api/jobs", s.apiJobs)
	mux.HandleFunc("/api/status", s.apiStatus)
	mux.HandleFunc("/api/enqueue", s.apiEnqueue)
	mux.HandleFunc("/api/dlq/", s.apiDlqRetry)
	mux.HandleFunc("/api/config", s.apiConfig)
	return mux
}

func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(DashboardHTML()))
}

func (s *Server) apiJobs(w http.ResponseWriter, r *http.Request) {
	rows, err := s.st.ListJobsByState("")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	var jobs []types.Job
	for rows.Next() {
		j, err := store.ScanJob(rows)
		if err != nil {
			continue
		}
		jobs = append(jobs, j)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jobs)
}

func (s *Server) apiStatus(w http.ResponseWriter, r *http.Request) {
	rows, err := s.st.CountJobsByState()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	counts := map[string]int{}
	for rows.Next() {
		var state string
		var count int
		rows.Scan(&state, &count)
		counts[state] = count
	}
	live, _ := s.st.LiveWorkerCount()
	counts["live_workers"] = live

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(counts)
}

func (s *Server) apiEnqueue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", 405)
		return
	}
	body, _ := io.ReadAll(r.Body)
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(fmt.Sprintf(`{"status":"ok","body":"%s"}`, body)))
}

func (s *Server) apiDlqRetry(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/dlq/")
	ok, err := s.st.RetryDlqJob(id)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	json.NewEncoder(w).Encode(map[string]bool{"retried": ok})
}

func (s *Server) apiConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		r.ParseForm()
		key := r.FormValue("key")
		val := r.FormValue("value")
		s.st.SetConfig(key, val)
	}
	maxRetries, _ := s.st.GetConfig("max-retries")
	backoffBase, _ := s.st.GetConfig("backoff-base")
	json.NewEncoder(w).Encode(map[string]string{
		"max-retries":  maxRetries,
		"backoff-base": backoffBase,
	})
}
