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
	"fizrmm-cli/internal/enrollment"
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
	mux.HandleFunc("/bootstrap/linux", bootstrapLinuxHandler(db, hs, cfg))

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

func bootstrapLinuxHandler(db *sql.DB, hs *headscale.Client, cfg config.API) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		if cfg.BootstrapToken == "" {
			writeError(w, http.StatusServiceUnavailable, "bootstrap_disabled", "bootstrap token is not configured")
			return
		}
		if !constantTimeEqual(r.URL.Query().Get("token"), cfg.BootstrapToken) {
			writeError(w, http.StatusUnauthorized, "invalid_bootstrap_token", "invalid bootstrap token")
			return
		}
		if !hs.Configured() {
			writeError(w, http.StatusServiceUnavailable, "headscale_unconfigured", "Headscale URL and API key are required")
			return
		}

		user := strings.TrimSpace(r.URL.Query().Get("user"))
		if user == "" {
			user = "lab"
		}
		ttl := 1 * time.Hour
		if requestedTTL := r.URL.Query().Get("ttl"); requestedTTL != "" {
			parsed, err := time.ParseDuration(requestedTTL)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid_ttl", err.Error())
				return
			}
			ttl = parsed
		}
		if ttl <= 0 || ttl > 24*time.Hour {
			writeError(w, http.StatusBadRequest, "invalid_ttl", "ttl must be greater than 0 and no more than 24h")
			return
		}

		loginServer := strings.TrimSpace(r.URL.Query().Get("login_server"))
		if loginServer == "" {
			loginServer = strings.TrimRight(cfg.PublicBaseURL, "/")
			loginServer = strings.TrimSuffix(loginServer, ":8080") + ":8081"
		}
		hostname := strings.TrimSpace(r.URL.Query().Get("hostname"))
		tags := splitQueryCSV(r.URL.Query().Get("tags"))
		if len(tags) == 0 {
			tags = []string{"tag:rmm-agent"}
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		key, err := hs.CreatePreAuthKey(ctx, headscale.CreatePreAuthKeyRequest{
			User:       user,
			Reusable:   false,
			Ephemeral:  false,
			Expiration: time.Now().UTC().Add(ttl),
			ACLTags:    tags,
		})
		if err != nil {
			writeError(w, http.StatusBadGateway, "headscale_preauthkey_create_failed", err.Error())
			return
		}

		actor := "bootstrap:" + clientIP(r)
		if db != nil {
			tenantID, err := store.EnsureTenant(ctx, db, user)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "tenant_record_failed", err.Error())
				return
			}
			if err := store.RecordEnrollmentKey(ctx, db, store.EnrollmentKeyInput{
				TenantID:       &tenantID,
				HeadscaleKeyID: key.ID,
				Key:            key.Key,
				UserName:       user,
				Tags:           tags,
				Reusable:       false,
				Ephemeral:      false,
				ExpiresAt:      key.Expiration,
				CreatedBy:      actor,
			}); err != nil {
				writeError(w, http.StatusInternalServerError, "enrollment_key_record_failed", err.Error())
				return
			}
			if err := store.RecordAuditEvent(ctx, db, &tenantID, actor, "bootstrap.linux", "headscale_preauthkey", key.ID, map[string]any{
				"user":        user,
				"tags":        tags,
				"expiresAt":   key.Expiration,
				"loginServer": loginServer,
			}); err != nil {
				writeError(w, http.StatusInternalServerError, "audit_record_failed", err.Error())
				return
			}
		}

		w.Header().Set("Content-Type", "text/x-shellscript; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, enrollment.LinuxScript(loginServer, key.Key, hostname))
	}
}

func healthzHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func splitQueryCSV(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func clientIP(r *http.Request) string {
	forwardedFor := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if forwardedFor != "" {
		parts := strings.Split(forwardedFor, ",")
		return strings.TrimSpace(parts[0])
	}
	return r.RemoteAddr
}

func constantTimeEqual(a string, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var result byte
	for i := range a {
		result |= a[i] ^ b[i]
	}
	return result == 0
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
