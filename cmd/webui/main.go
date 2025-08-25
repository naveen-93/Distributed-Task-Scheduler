package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	pb "distributed-task-scheduler/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

//go:embed static/*
var staticFS embed.FS

type submitRequest struct {
	Command      string `json:"command"`
	SubmitServer string `json:"submitServer"`
}

type submitResponse struct {
	JobID string `json:"jobId"`
	Error string `json:"error,omitempty"`
}

type statusResponse struct {
	Status string `json:"status"`
	Output string `json:"output"`
	Error  string `json:"error,omitempty"`
}

func main() {
	addr := os.Getenv("WEBUI_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	// Static files
	sub, _ := fs.Sub(staticFS, "static")
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(sub))))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// serve embedded index.html
		b, err := staticFS.ReadFile(filepath.Join("static", "index.html"))
		if err != nil {
			http.Error(w, "index not found", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(b)
	})

	// API endpoints
	http.HandleFunc("/servers", handleServers)
	http.HandleFunc("/submit", handleSubmit)
	http.HandleFunc("/status", handleStatus)

	log.Printf("Web UI listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func handleServers(w http.ResponseWriter, r *http.Request) {
	servers := resolveServers()
	writeJSON(w, servers)
}

func handleSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req submitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Command) == "" || strings.TrimSpace(req.SubmitServer) == "" {
		http.Error(w, "command and submitServer are required", http.StatusBadRequest)
		return
	}
	ctx := context.Background()
	conn, err := grpc.Dial(req.SubmitServer, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		writeJSON(w, submitResponse{Error: fmt.Sprintf("dial error: %v", err)})
		return
	}
	defer conn.Close()
	client := pb.NewJobServiceClient(conn)
	job := &pb.Job{Command: req.Command, CreatedAt: time.Now().Unix()}
	resp, err := client.SubmitJob(ctx, job)
	if err != nil {
		writeJSON(w, submitResponse{Error: fmt.Sprintf("submit error: %v", err)})
		return
	}
	writeJSON(w, submitResponse{JobID: resp.JobId})
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	jobID := r.URL.Query().Get("jobId")
	server := r.URL.Query().Get("server")
	if strings.TrimSpace(jobID) == "" || strings.TrimSpace(server) == "" {
		http.Error(w, "jobId and server query params are required", http.StatusBadRequest)
		return
	}
	ctx := context.Background()
	conn, err := grpc.Dial(server, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		writeJSON(w, statusResponse{Error: fmt.Sprintf("dial error: %v", err)})
		return
	}
	defer conn.Close()
	client := pb.NewJobServiceClient(conn)
	st, err := client.GetJobStatus(ctx, &pb.JobId{Id: jobID})
	if err != nil {
		writeJSON(w, statusResponse{Error: fmt.Sprintf("status error: %v", err)})
		return
	}
	writeJSON(w, statusResponse{Status: st.Status, Output: st.Output})
}

func resolveServers() []string {
	// Order of resolution: SERVERS env, .servers file, default
	if env := os.Getenv("SERVERS"); env != "" {
		return splitAndTrim(env)
	}
	if b, err := os.ReadFile(".servers"); err == nil {
		return splitAndTrim(string(b))
	}
	return []string{"localhost:50051"}
}

func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
