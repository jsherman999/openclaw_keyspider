package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jsherman999/openclaw_keyspider/internal/config"
	"github.com/jsherman999/openclaw_keyspider/internal/db"
	"github.com/jsherman999/openclaw_keyspider/internal/store"
)

type API struct {
	cfg   *config.Config
	db    *db.DB
	store *store.Store
}

func New(cfg *config.Config, dbc *db.DB) *API {
	return &API{cfg: cfg, db: dbc, store: store.New(dbc)}
}

func (a *API) Router() http.Handler {
	r := chi.NewRouter()

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
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

	return r
}
