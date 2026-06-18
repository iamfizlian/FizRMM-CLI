package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"fizrmm-cli/internal/config"
	"fizrmm-cli/internal/headscale"
	"fizrmm-cli/internal/store"
	"fizrmm-cli/internal/version"
)

func main() {
	cfg := config.LoadAPI()
	addr := flag.String("addr", cfg.Addr, "listen address")
	migrate := flag.Bool("migrate", false, "apply database migrations and exit")
	showVersion := flag.Bool("version", false, "print version")
	flag.Parse()

	if *showVersion {
		fmt.Fprintf(os.Stdout, "%s API %s\n", version.Name, version.Version)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var db *sql.DB
	hs := headscale.New(cfg.HeadscaleURL, cfg.HeadscaleKey)
	if cfg.DatabaseURL != "" {
		var err error
		db, err = store.Open(ctx, cfg.DatabaseURL)
		if err != nil {
			log.Fatalf("database connection failed: %v", err)
		}
		defer db.Close()

		if *migrate || cfg.AutoMigrate {
			if err := store.Migrate(ctx, db, cfg.MigrationsDir); err != nil {
				log.Fatalf("database migration failed: %v", err)
			}
			log.Printf("database migrations applied from %s", cfg.MigrationsDir)
		}
	}

	if *migrate {
		if cfg.DatabaseURL == "" {
			log.Fatal("RMM_DATABASE_URL is required for --migrate")
		}
		return
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthzHandler)
	mux.HandleFunc("/metrics", metricsHandler)
	mux.HandleFunc("/v1/version", versionHandler)
	mux.HandleFunc("/v1/tenants", emptyListHandler("tenants"))
	mux.HandleFunc("/v1/sites", emptyListHandler("sites"))
	mux.HandleFunc("/v1/nodes", nodesHandler(db))
	mux.HandleFunc("/v1/jobs", emptyListHandler("jobs"))
	mux.HandleFunc("/v1/alerts", emptyListHandler("alerts"))
	mux.HandleFunc("/v1/audit-events", emptyListHandler("audit_events"))
	mux.HandleFunc("/v1/overlay/nodes", overlayNodesHandler(db, hs))
	mux.HandleFunc("/v1/overlay/preauthkeys", overlayPreAuthKeysHandler(db, hs))

	server := &http.Server{
		Addr:              *addr,
		Handler:           requestLogger(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("starting rmm-api on %s", *addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("rmm-api failed: %v", err)
	}
}

func healthzHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func versionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"name":    version.Name,
		"version": version.Version,
	})
}

func metricsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	_, _ = fmt.Fprintln(w, "# HELP rmm_api_up RMM API availability")
	_, _ = fmt.Fprintln(w, "# TYPE rmm_api_up gauge")
	_, _ = fmt.Fprintln(w, "rmm_api_up 1")
}

func emptyListHandler(resource string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"data": []any{},
			"meta": map[string]any{
				"resource": resource,
				"count":    0,
			},
		})
	}
}

func nodesHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}

		if db == nil {
			writeJSON(w, http.StatusOK, map[string]any{
				"data": []any{},
				"meta": map[string]any{
					"resource": "nodes",
					"count":    0,
					"source":   "unconfigured_database",
				},
			})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		nodes, err := store.ListNodes(ctx, db)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "node_list_failed", err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"data": nodes,
			"meta": map[string]any{
				"resource": "nodes",
				"count":    len(nodes),
				"source":   "postgres",
			},
		})
	}
}

func overlayNodesHandler(db *sql.DB, hs *headscale.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		if !hs.Configured() {
			writeError(w, http.StatusServiceUnavailable, "headscale_unconfigured", "Headscale URL and API key are required")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		nodes, err := hs.ListNodes(ctx)
		if err != nil {
			writeError(w, http.StatusBadGateway, "headscale_node_list_failed", err.Error())
			return
		}

		meta := map[string]any{
			"resource": "overlay_nodes",
			"count":    len(nodes),
			"source":   "headscale",
		}

		if r.Method == http.MethodPost {
			if db == nil {
				writeError(w, http.StatusServiceUnavailable, "database_unconfigured", "database is required for sync")
				return
			}
			synced, err := store.SyncHeadscaleNodes(ctx, db, nodes)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "headscale_node_sync_failed", err.Error())
				return
			}
			meta["synced"] = synced
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"data": nodes,
			"meta": meta,
		})
	}
}

func overlayPreAuthKeysHandler(db *sql.DB, hs *headscale.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		if !hs.Configured() {
			writeError(w, http.StatusServiceUnavailable, "headscale_unconfigured", "Headscale URL and API key are required")
			return
		}

		var request struct {
			User      string   `json:"user"`
			TTL       string   `json:"ttl"`
			Reusable  bool     `json:"reusable"`
			Ephemeral bool     `json:"ephemeral"`
			Tags      []string `json:"tags"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}

		request.User = strings.TrimSpace(request.User)
		if request.User == "" {
			writeError(w, http.StatusBadRequest, "missing_user", "user is required")
			return
		}
		if request.TTL == "" {
			request.TTL = "1h"
		}
		ttl, err := time.ParseDuration(request.TTL)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_ttl", err.Error())
			return
		}
		if ttl <= 0 || ttl > 24*time.Hour {
			writeError(w, http.StatusBadRequest, "invalid_ttl", "ttl must be greater than 0 and no more than 24h")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		key, err := hs.CreatePreAuthKey(ctx, headscale.CreatePreAuthKeyRequest{
			User:       request.User,
			Reusable:   request.Reusable,
			Ephemeral:  request.Ephemeral,
			Expiration: time.Now().UTC().Add(ttl),
			ACLTags:    request.Tags,
		})
		if err != nil {
			writeError(w, http.StatusBadGateway, "headscale_preauthkey_create_failed", err.Error())
			return
		}

		actor := strings.TrimSpace(r.Header.Get("X-RMM-Actor"))
		if actor == "" {
			actor = "system:anonymous"
		}
		if db != nil {
			tenantID, err := store.EnsureTenant(ctx, db, request.User)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "tenant_record_failed", err.Error())
				return
			}
			if err := store.RecordEnrollmentKey(ctx, db, store.EnrollmentKeyInput{
				TenantID:       &tenantID,
				HeadscaleKeyID: key.ID,
				Key:            key.Key,
				UserName:       request.User,
				Tags:           request.Tags,
				Reusable:       request.Reusable,
				Ephemeral:      request.Ephemeral,
				ExpiresAt:      key.Expiration,
				CreatedBy:      actor,
			}); err != nil {
				writeError(w, http.StatusInternalServerError, "enrollment_key_record_failed", err.Error())
				return
			}
			if err := store.RecordAuditEvent(ctx, db, &tenantID, actor, "overlay.preauthkey.create", "headscale_preauthkey", key.ID, map[string]any{
				"user":      request.User,
				"tags":      request.Tags,
				"reusable":  request.Reusable,
				"ephemeral": request.Ephemeral,
				"expiresAt": key.Expiration,
			}); err != nil {
				writeError(w, http.StatusInternalServerError, "audit_record_failed", err.Error())
				return
			}
		}

		writeJSON(w, http.StatusCreated, map[string]any{
			"data": key,
			"meta": map[string]any{
				"resource": "overlay_preauthkey",
				"source":   "headscale",
			},
		})
	}
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code string, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}
