package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"fizrmm-cli/internal/headscale"

	_ "github.com/jackc/pgx/v5/stdlib"
)

var nonSlugChars = regexp.MustCompile(`[^a-z0-9-]+`)

type Node struct {
	ID              int64           `json:"id"`
	TenantID        int64           `json:"tenant_id"`
	SiteID          *int64          `json:"site_id,omitempty"`
	Hostname        string          `json:"hostname"`
	FQDN            *string         `json:"fqdn,omitempty"`
	OSFamily        *string         `json:"os_family,omitempty"`
	OSVersion       *string         `json:"os_version,omitempty"`
	Architecture    *string         `json:"architecture,omitempty"`
	HeadscaleNodeID *string         `json:"headscale_node_id,omitempty"`
	TailnetIP       *string         `json:"tailnet_ip,omitempty"`
	LastSeenAt      *time.Time      `json:"last_seen_at,omitempty"`
	Status          string          `json:"status"`
	Tags            json.RawMessage `json:"tags"`
	CreatedAt       time.Time       `json:"created_at"`
}

type EnrollmentKeyInput struct {
	TenantID       *int64
	HeadscaleKeyID string
	Key            string
	UserName       string
	Tags           []string
	Reusable       bool
	Ephemeral      bool
	ExpiresAt      *time.Time
	CreatedBy      string
}

type CommandJobInput struct {
	TenantID *int64
	NodeID   int64
	Actor    string
	Command  string
	Reason   string
}

type CommandJobResultInput struct {
	JobID      int64
	NodeID     int64
	ExitCode   int
	Stdout     string
	Stderr     string
	StartedAt  time.Time
	FinishedAt time.Time
}

func Open(ctx context.Context, databaseURL string) (*sql.DB, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func Migrate(ctx context.Context, db *sql.DB, migrationsDir string) error {
	if _, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
  version TEXT PRIMARY KEY,
  applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		files = append(files, entry.Name())
	}
	sort.Strings(files)

	for _, name := range files {
		applied, err := migrationApplied(ctx, db, name)
		if err != nil {
			return err
		}
		if applied {
			continue
		}

		if err := applyMigration(ctx, db, migrationsDir, name); err != nil {
			return err
		}
	}

	return nil
}

func ListNodes(ctx context.Context, db *sql.DB) ([]Node, error) {
	rows, err := db.QueryContext(ctx, `
SELECT
  id,
  tenant_id,
  site_id,
  hostname,
  fqdn,
  os_family,
  os_version,
  architecture,
  headscale_node_id,
  tailnet_ip::text,
  last_seen_at,
  status,
  tags,
  created_at
FROM nodes
ORDER BY hostname`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	nodes := make([]Node, 0)
	for rows.Next() {
		var node Node
		if err := rows.Scan(
			&node.ID,
			&node.TenantID,
			&node.SiteID,
			&node.Hostname,
			&node.FQDN,
			&node.OSFamily,
			&node.OSVersion,
			&node.Architecture,
			&node.HeadscaleNodeID,
			&node.TailnetIP,
			&node.LastSeenAt,
			&node.Status,
			&node.Tags,
			&node.CreatedAt,
		); err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return nodes, nil
}

func FindNode(ctx context.Context, db *sql.DB, identifier string) (Node, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return Node{}, fmt.Errorf("node identifier is required")
	}

	where := "hostname = $1"
	if _, err := strconv.ParseInt(identifier, 10, 64); err == nil {
		where = "id = $1"
	}

	row := db.QueryRowContext(ctx, `
SELECT
  id,
  tenant_id,
  site_id,
  hostname,
  fqdn,
  os_family,
  os_version,
  architecture,
  headscale_node_id,
  tailnet_ip::text,
  last_seen_at,
  status,
  tags,
  created_at
FROM nodes
WHERE `+where+`
ORDER BY id
LIMIT 1`, identifier)

	var node Node
	if err := row.Scan(
		&node.ID,
		&node.TenantID,
		&node.SiteID,
		&node.Hostname,
		&node.FQDN,
		&node.OSFamily,
		&node.OSVersion,
		&node.Architecture,
		&node.HeadscaleNodeID,
		&node.TailnetIP,
		&node.LastSeenAt,
		&node.Status,
		&node.Tags,
		&node.CreatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return Node{}, fmt.Errorf("node %q not found", identifier)
		}
		return Node{}, err
	}
	return node, nil
}

func SyncHeadscaleNodes(ctx context.Context, db *sql.DB, headscaleNodes []headscale.Node) (int, error) {
	count := 0
	for _, node := range headscaleNodes {
		tenantName := node.User.Name
		if tenantName == "" {
			tenantName = "headscale"
		}

		tenantID, err := EnsureTenant(ctx, db, tenantName)
		if err != nil {
			return count, err
		}

		if err := UpsertHeadscaleNode(ctx, db, tenantID, node); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func EnsureTenant(ctx context.Context, db *sql.DB, name string) (int64, error) {
	slug := slugify(name)

	var id int64
	err := db.QueryRowContext(ctx, `
INSERT INTO tenants (name, slug)
VALUES ($1, $2)
ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name
RETURNING id`, name, slug).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("ensure tenant %s: %w", slug, err)
	}
	return id, nil
}

func UpsertHeadscaleNode(ctx context.Context, db *sql.DB, tenantID int64, node headscale.Node) error {
	if node.ID == "" {
		return fmt.Errorf("headscale node missing id")
	}

	hostname := node.GivenName
	if hostname == "" {
		hostname = node.Name
	}
	if hostname == "" {
		hostname = "headscale-node-" + node.ID
	}

	status := "offline"
	if node.Online {
		status = "online"
	}

	var tailnetIP *string
	if len(node.IPAddresses) > 0 {
		tailnetIP = &node.IPAddresses[0]
	}

	tags := append([]string{}, node.ValidTags...)
	tags = append(tags, node.ForcedTags...)
	tagsJSON, err := json.Marshal(tags)
	if err != nil {
		return err
	}

	_, err = db.ExecContext(ctx, `
INSERT INTO nodes (
  tenant_id,
  hostname,
  headscale_node_id,
  tailnet_ip,
  last_seen_at,
  status,
  tags
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (headscale_node_id) DO UPDATE SET
  tenant_id = EXCLUDED.tenant_id,
  hostname = EXCLUDED.hostname,
  tailnet_ip = EXCLUDED.tailnet_ip,
  last_seen_at = EXCLUDED.last_seen_at,
  status = EXCLUDED.status,
  tags = EXCLUDED.tags`, tenantID, hostname, node.ID, tailnetIP, node.LastSeen, status, tagsJSON)
	if err != nil {
		return fmt.Errorf("upsert headscale node %s: %w", node.ID, err)
	}
	return nil
}

func RecordEnrollmentKey(ctx context.Context, db *sql.DB, input EnrollmentKeyInput) error {
	tagsJSON, err := json.Marshal(input.Tags)
	if err != nil {
		return err
	}

	keyHash := sha256.Sum256([]byte(input.Key))
	_, err = db.ExecContext(ctx, `
INSERT INTO enrollment_keys (
  tenant_id,
  headscale_key_id,
  key_hash,
  user_name,
  tags,
  reusable,
  ephemeral,
  expires_at,
  created_by
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		input.TenantID,
		input.HeadscaleKeyID,
		hex.EncodeToString(keyHash[:]),
		input.UserName,
		tagsJSON,
		input.Reusable,
		input.Ephemeral,
		input.ExpiresAt,
		input.CreatedBy,
	)
	if err != nil {
		return fmt.Errorf("record enrollment key: %w", err)
	}
	return nil
}

func RecordAuditEvent(ctx context.Context, db *sql.DB, tenantID *int64, actor string, action string, targetType string, targetID string, metadata map[string]any) error {
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return err
	}

	_, err = db.ExecContext(ctx, `
INSERT INTO audit_events (
  tenant_id,
  actor,
  action,
  target_type,
  target_id,
  metadata
)
VALUES ($1, $2, $3, $4, $5, $6)`, tenantID, actor, action, targetType, targetID, metadataJSON)
	if err != nil {
		return fmt.Errorf("record audit event: %w", err)
	}
	return nil
}

func CreateCommandJob(ctx context.Context, db *sql.DB, input CommandJobInput) (int64, error) {
	var id int64
	err := db.QueryRowContext(ctx, `
INSERT INTO jobs (
  tenant_id,
  created_by,
  type,
  status,
  target_selector,
  command_or_playbook,
  reason,
  started_at
)
VALUES ($1, $2, 'ssh_command', 'running', $3, $4, $5, now())
RETURNING id`,
		input.TenantID,
		input.Actor,
		fmt.Sprintf("node:%d", input.NodeID),
		input.Command,
		input.Reason,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("create command job: %w", err)
	}
	return id, nil
}

func FinishCommandJob(ctx context.Context, db *sql.DB, jobID int64, status string) error {
	_, err := db.ExecContext(ctx, `
UPDATE jobs
SET status = $2, finished_at = now()
WHERE id = $1`, jobID, status)
	if err != nil {
		return fmt.Errorf("finish command job: %w", err)
	}
	return nil
}

func RecordCommandJobResult(ctx context.Context, db *sql.DB, input CommandJobResultInput) error {
	_, err := db.ExecContext(ctx, `
INSERT INTO job_results (
  job_id,
  node_id,
  exit_code,
  stdout_ref,
  stderr_ref,
  started_at,
  finished_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		input.JobID,
		input.NodeID,
		input.ExitCode,
		input.Stdout,
		input.Stderr,
		input.StartedAt,
		input.FinishedAt,
	)
	if err != nil {
		return fmt.Errorf("record command result: %w", err)
	}
	return nil
}

func migrationApplied(ctx context.Context, db *sql.DB, version string) (bool, error) {
	var exists bool
	if err := db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`, version).Scan(&exists); err != nil {
		return false, fmt.Errorf("check migration %s: %w", version, err)
	}
	return exists, nil
}

func applyMigration(ctx context.Context, db *sql.DB, migrationsDir string, name string) error {
	path := filepath.Join(migrationsDir, name)
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read migration %s: %w", name, err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %s: %w", name, err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if _, err := tx.ExecContext(ctx, string(content)); err != nil {
		return fmt.Errorf("apply migration %s: %w", name, err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, name); err != nil {
		return fmt.Errorf("record migration %s: %w", name, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %s: %w", name, err)
	}

	return nil
}

func slugify(value string) string {
	slug := strings.ToLower(strings.TrimSpace(value))
	slug = strings.ReplaceAll(slug, "_", "-")
	slug = nonSlugChars.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		return "default"
	}
	return slug
}
