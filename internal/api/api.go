package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jsherman999/openclaw_keyspider/internal/config"
	"github.com/jsherman999/openclaw_keyspider/internal/db"
	"github.com/jsherman999/openclaw_keyspider/internal/exporter"
	"github.com/jsherman999/openclaw_keyspider/internal/store"
	"github.com/jsherman999/openclaw_keyspider/internal/watchhub"
	"github.com/jsherman999/openclaw_keyspider/internal/webui"
)

type API struct {
	cfg   *config.Config
	db    *db.DB
	store *store.Store
	hub   *watchhub.Hub
}

func New(cfg *config.Config, dbc *db.DB, hub *watchhub.Hub) *API {
	return &API{cfg: cfg, db: dbc, store: store.New(dbc), hub: hub}
}

func (a *API) Router() http.Handler {
	r := chi.NewRouter()

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// Enqueue a scan job (used by the Web UI)
	// POST /scan {"host":"server","since_seconds":604800,"spider_depth":1}
	r.Post("/scan", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Host         string `json:"host"`
			SinceSeconds int    `json:"since_seconds"`
			SpiderDepth  int    `json:"spider_depth"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", 400)
			return
		}
		if req.Host == "" {
			http.Error(w, "host required", 400)
			return
		}
		since := 168 * 3600
		if req.SinceSeconds > 0 {
			since = req.SinceSeconds
		}
		jobID, err := a.store.EnqueueScanJob(r.Context(), req.Host, time.Duration(since)*time.Second, req.SpiderDepth)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"job_id": jobID})
	})

	// Get scan job status
	r.Get("/scan/{id}", func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "bad id", 400)
			return
		}
		job, err := a.store.GetScanJob(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		_ = json.NewEncoder(w).Encode(job)
	})

	// Phase 3: SSE stream of newly-ingested watcher events.
	r.Get("/watch/events", func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", 500)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		ch := a.hub.Subscribe(256)
		defer a.hub.Unsubscribe(ch)

		// send a comment to open stream
		_, _ = w.Write([]byte(": ok\n\n"))
		flusher.Flush()

		for {
			select {
			case <-r.Context().Done():
				return
			case b := <-ch:
				_, _ = w.Write([]byte("data: "))
				_, _ = w.Write(b)
				_, _ = w.Write([]byte("\n\n"))
				flusher.Flush()
			}
		}
	})

	r.Get("/hosts", func(w http.ResponseWriter, r *http.Request) {
		hosts, err := a.store.ListHosts(r.Context(), 200)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		_ = json.NewEncoder(w).Encode(hosts)
	})

	r.Get("/events", func(w http.ResponseWriter, r *http.Request) {
		hostIDStr := r.URL.Query().Get("host_id")
		if hostIDStr == "" {
			http.Error(w, "host_id required", 400)
			return
		}
		hid, err := strconv.ParseInt(hostIDStr, 10, 64)
		if err != nil {
			http.Error(w, "bad host_id", 400)
			return
		}
		events, err := a.store.ListAccessEvents(r.Context(), hid, 500)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		_ = json.NewEncoder(w).Encode(events)
	})

	// Phase 4 (exports only): download graph export.
	// GET /export/graph?format=json|csv|graphml
	r.Get("/export/graph", func(w http.ResponseWriter, r *http.Request) {
		format := r.URL.Query().Get("format")
		if format == "" {
			format = "json"
		}
		limit := 10000
		if l := r.URL.Query().Get("limit"); l != "" {
			if v, err := strconv.Atoi(l); err == nil {
				limit = v
			}
		}

		var (
			b   []byte
			ct  string
			err error
		)
		switch format {
		case "json":
			b, ct, err = exporter.ExportGraphJSON(r.Context(), a.store, limit)
		case "csv":
			b, ct, err = exporter.ExportGraphCSV(r.Context(), a.store, limit)
		case "graphml":
			b, ct, err = exporter.ExportGraphGraphML(r.Context(), a.store, limit)
		default:
			http.Error(w, "unknown format", 400)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", ct)
		w.WriteHeader(200)
		_, _ = w.Write(b)
	})

	// Web UI
	ui, uiErr := webui.Handler()
	if uiErr == nil {
		r.Handle("/*", ui)
	}

	return r
}
