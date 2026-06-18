package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"fizrmm-cli/internal/version"
)

type command struct {
	Name        string
	Description string
}

var commands = []command{
	{Name: "tenant list", Description: "List tenants"},
	{Name: "site list", Description: "List sites"},
	{Name: "node list", Description: "List managed nodes"},
	{Name: "node enroll-script", Description: "Generate an endpoint enrollment script"},
	{Name: "node ssh", Description: "Open an audited SSH session"},
	{Name: "exec", Description: "Run an audited command"},
	{Name: "job run", Description: "Run a playbook or approved job"},
	{Name: "alerts list", Description: "List active alerts"},
	{Name: "overlay nodes", Description: "List overlay nodes"},
	{Name: "overlay nodes sync", Description: "Sync overlay nodes into inventory"},
	{Name: "overlay preauth create", Description: "Create a Headscale pre-auth key"},
}

func main() {
	apiURL := flag.String("api-url", envOrDefault("RMM_API_URL", "http://localhost:8080"), "RMM API base URL")
	jsonOutput := flag.Bool("json", false, "emit JSON output")
	showVersion := flag.Bool("version", false, "print version")
	flag.Usage = usage
	flag.Parse()

	if *showVersion {
		printVersion(*jsonOutput)
		return
	}

	args := flag.Args()
	if len(args) == 0 || args[0] == "help" {
		usage()
		return
	}

	commandName := strings.Join(args, " ")
	switch {
	case commandName == "node list":
		if err := listNodes(*apiURL, *jsonOutput); err != nil {
			fmt.Fprintf(os.Stderr, "node list failed: %v\n", err)
			os.Exit(1)
		}
	case len(args) >= 2 && args[0] == "node" && args[1] == "enroll-script":
		if err := generateEnrollScript(*apiURL, args[2:], *jsonOutput); err != nil {
			fmt.Fprintf(os.Stderr, "node enroll-script failed: %v\n", err)
			os.Exit(1)
		}
	case commandName == "overlay nodes":
		if err := overlayNodes(*apiURL, false, *jsonOutput); err != nil {
			fmt.Fprintf(os.Stderr, "overlay nodes failed: %v\n", err)
			os.Exit(1)
		}
	case commandName == "overlay nodes sync":
		if err := overlayNodes(*apiURL, true, *jsonOutput); err != nil {
			fmt.Fprintf(os.Stderr, "overlay nodes sync failed: %v\n", err)
			os.Exit(1)
		}
	case len(args) >= 3 && args[0] == "overlay" && args[1] == "preauth" && args[2] == "create":
		if err := createPreAuthKey(*apiURL, args[3:], *jsonOutput); err != nil {
			fmt.Fprintf(os.Stderr, "overlay preauth create failed: %v\n", err)
			os.Exit(1)
		}
	case commandName == "tenant list" || commandName == "site list" || commandName == "alerts list":
		printNotImplemented(args, *jsonOutput)
	default:
		fmt.Fprintf(os.Stderr, "unknown or unavailable command: %s\n\n", strings.Join(args, " "))
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintf(os.Stdout, "%s terminal RMM CLI\n\n", version.Name)
	fmt.Fprintln(os.Stdout, "Usage:")
	fmt.Fprintln(os.Stdout, "  rmmctl [--api-url URL] [--json] [--version] <command>")
	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, "Commands:")
	for _, cmd := range commands {
		fmt.Fprintf(os.Stdout, "  %-16s %s\n", cmd.Name, cmd.Description)
	}
}

func printVersion(jsonOutput bool) {
	if jsonOutput {
		_ = json.NewEncoder(os.Stdout).Encode(map[string]string{
			"name":    version.Name,
			"version": version.Version,
		})
		return
	}
	fmt.Fprintf(os.Stdout, "%s %s\n", version.Name, version.Version)
}

func printNotImplemented(args []string, jsonOutput bool) {
	name := strings.Join(args, " ")
	if jsonOutput {
		_ = json.NewEncoder(os.Stdout).Encode(map[string]string{
			"command": name,
			"status":  "not_implemented",
		})
		return
	}
	fmt.Fprintf(os.Stdout, "%s: not implemented yet\n", name)
}

type nodeListResponse struct {
	Data []node `json:"data"`
	Meta any    `json:"meta"`
}

type overlayNodeListResponse struct {
	Data []overlayNode  `json:"data"`
	Meta map[string]any `json:"meta"`
}

type overlayNode struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	GivenName   string     `json:"givenName"`
	User        user       `json:"user"`
	IPAddresses []string   `json:"ipAddresses"`
	Online      bool       `json:"online"`
	LastSeen    *time.Time `json:"lastSeen,omitempty"`
	ValidTags   []string   `json:"validTags,omitempty"`
	ForcedTags  []string   `json:"forcedTags,omitempty"`
}

type user struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type preAuthKeyResponse struct {
	Data preAuthKey `json:"data"`
	Meta any        `json:"meta"`
}

type preAuthKey struct {
	User       string     `json:"user"`
	ID         string     `json:"id"`
	Key        string     `json:"key"`
	Reusable   bool       `json:"reusable"`
	Ephemeral  bool       `json:"ephemeral"`
	Used       bool       `json:"used"`
	Expiration *time.Time `json:"expiration,omitempty"`
	ACLTags    []string   `json:"aclTags"`
}

type enrollmentScriptResponse struct {
	User        string   `json:"user"`
	OS          string   `json:"os"`
	LoginServer string   `json:"login_server"`
	Tags        []string `json:"tags,omitempty"`
	Key         string   `json:"key"`
	Script      string   `json:"script"`
}

type node struct {
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

func listNodes(apiURL string, jsonOutput bool) error {
	url := strings.TrimRight(apiURL, "/") + "/v1/nodes"
	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("api returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	if jsonOutput {
		fmt.Fprintln(os.Stdout, string(body))
		return nil
	}

	var decoded nodeListResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return err
	}

	if len(decoded.Data) == 0 {
		fmt.Fprintln(os.Stdout, "No nodes found.")
		return nil
	}

	fmt.Fprintf(os.Stdout, "%-6s %-24s %-12s %-16s %-20s\n", "ID", "HOSTNAME", "STATUS", "TENANT", "TAILNET IP")
	for _, node := range decoded.Data {
		fmt.Fprintf(os.Stdout, "%-6d %-24s %-12s %-16d %-20s\n",
			node.ID,
			node.Hostname,
			node.Status,
			node.TenantID,
			stringOrDash(node.TailnetIP),
		)
	}

	return nil
}

func overlayNodes(apiURL string, sync bool, jsonOutput bool) error {
	method := http.MethodGet
	if sync {
		method = http.MethodPost
	}

	body, err := apiRequest(apiURL, method, "/v1/overlay/nodes", nil)
	if err != nil {
		return err
	}

	if jsonOutput {
		fmt.Fprintln(os.Stdout, string(body))
		return nil
	}

	var decoded overlayNodeListResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return err
	}

	if len(decoded.Data) == 0 {
		if sync {
			fmt.Fprintln(os.Stdout, "No overlay nodes found. Synced 0 nodes.")
			return nil
		}
		fmt.Fprintln(os.Stdout, "No overlay nodes found.")
		return nil
	}

	fmt.Fprintf(os.Stdout, "%-8s %-24s %-16s %-8s %-20s\n", "ID", "NAME", "USER", "ONLINE", "IP")
	for _, node := range decoded.Data {
		fmt.Fprintf(os.Stdout, "%-8s %-24s %-16s %-8t %-20s\n",
			node.ID,
			firstNonEmpty(node.GivenName, node.Name),
			node.User.Name,
			node.Online,
			firstStringOrDash(node.IPAddresses),
		)
	}

	if sync {
		if synced, ok := decoded.Meta["synced"]; ok {
			fmt.Fprintf(os.Stdout, "Synced %v nodes.\n", synced)
		}
	}

	return nil
}

func createPreAuthKey(apiURL string, args []string, jsonOutput bool) error {
	flags := flag.NewFlagSet("overlay preauth create", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	user := flags.String("user", "", "Headscale user")
	ttl := flags.String("ttl", "1h", "key TTL")
	tags := flags.String("tags", "", "comma-separated tags")
	reusable := flags.Bool("reusable", false, "make key reusable")
	ephemeral := flags.Bool("ephemeral", false, "make key ephemeral")
	if err := flags.Parse(args); err != nil {
		return err
	}

	request := map[string]any{
		"user":      *user,
		"ttl":       *ttl,
		"reusable":  *reusable,
		"ephemeral": *ephemeral,
		"tags":      splitCSV(*tags),
	}

	body, err := apiRequest(apiURL, http.MethodPost, "/v1/overlay/preauthkeys", request)
	if err != nil {
		return err
	}

	if jsonOutput {
		fmt.Fprintln(os.Stdout, string(body))
		return nil
	}

	var decoded preAuthKeyResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "Key: %s\n", decoded.Data.Key)
	fmt.Fprintf(os.Stdout, "User: %s\n", decoded.Data.User)
	if decoded.Data.Expiration != nil {
		fmt.Fprintf(os.Stdout, "Expires: %s\n", decoded.Data.Expiration.Format(time.RFC3339))
	}
	return nil
}

func generateEnrollScript(apiURL string, args []string, jsonOutput bool) error {
	flags := flag.NewFlagSet("node enroll-script", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	osName := flags.String("os", "linux", "target OS: linux or windows")
	user := flags.String("user", "", "Headscale user")
	loginServer := flags.String("login-server", envOrDefault("RMM_HEADSCALE_LOGIN_SERVER", "http://localhost:8081"), "Headscale login server URL reachable by the endpoint")
	ttl := flags.String("ttl", "1h", "key TTL")
	tags := flags.String("tags", "tag:rmm-agent", "comma-separated tags")
	hostname := flags.String("hostname", "", "optional fixed hostname")
	reusable := flags.Bool("reusable", false, "make key reusable")
	ephemeral := flags.Bool("ephemeral", false, "make key ephemeral")
	if err := flags.Parse(args); err != nil {
		return err
	}

	normalizedOS := strings.ToLower(strings.TrimSpace(*osName))
	if normalizedOS != "linux" && normalizedOS != "windows" {
		return fmt.Errorf("unsupported os %q", *osName)
	}
	if strings.TrimSpace(*user) == "" {
		return fmt.Errorf("--user is required")
	}

	key, err := requestPreAuthKey(apiURL, *user, *ttl, splitCSV(*tags), *reusable, *ephemeral)
	if err != nil {
		return err
	}

	script := linuxEnrollScript(*loginServer, key.Key, *hostname)
	if normalizedOS == "windows" {
		script = windowsEnrollScript(*loginServer, key.Key, *hostname)
	}

	if jsonOutput {
		_ = json.NewEncoder(os.Stdout).Encode(enrollmentScriptResponse{
			User:        *user,
			OS:          normalizedOS,
			LoginServer: *loginServer,
			Tags:        splitCSV(*tags),
			Key:         key.Key,
			Script:      script,
		})
		return nil
	}

	fmt.Fprint(os.Stdout, script)
	return nil
}

func requestPreAuthKey(apiURL string, user string, ttl string, tags []string, reusable bool, ephemeral bool) (preAuthKey, error) {
	request := map[string]any{
		"user":      user,
		"ttl":       ttl,
		"reusable":  reusable,
		"ephemeral": ephemeral,
		"tags":      tags,
	}

	body, err := apiRequest(apiURL, http.MethodPost, "/v1/overlay/preauthkeys", request)
	if err != nil {
		return preAuthKey{}, err
	}

	var decoded preAuthKeyResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return preAuthKey{}, err
	}
	return decoded.Data, nil
}

func apiRequest(apiURL string, method string, path string, requestBody any) ([]byte, error) {
	url := strings.TrimRight(apiURL, "/") + path
	client := &http.Client{Timeout: 10 * time.Second}

	var body io.Reader
	if requestBody != nil {
		encoded, err := json.Marshal(requestBody)
		if err != nil {
			return nil, err
		}
		body = strings.NewReader(string(encoded))
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	if requestBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("X-RMM-Actor", envOrDefault("RMM_ACTOR", envOrDefault("USER", "local-operator")))

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	response, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("api returned %s: %s", resp.Status, strings.TrimSpace(string(response)))
	}

	return response, nil
}

func linuxEnrollScript(loginServer string, authKey string, hostname string) string {
	hostnameLine := `HOSTNAME="$(hostname -f 2>/dev/null || hostname)"`
	if strings.TrimSpace(hostname) != "" {
		hostnameLine = fmt.Sprintf("HOSTNAME=%q", hostname)
	}

	return fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail

LOGIN_SERVER=%q
AUTH_KEY=%q
%s

if ! command -v tailscale >/dev/null 2>&1; then
  curl -fsSL https://tailscale.com/install.sh | sh
fi

if command -v systemctl >/dev/null 2>&1; then
  sudo systemctl enable --now tailscaled
fi

sudo tailscale up \
  --login-server "${LOGIN_SERVER}" \
  --authkey "${AUTH_KEY}" \
  --hostname "${HOSTNAME}" \
  --ssh=false

tailscale status
`, loginServer, authKey, hostnameLine)
}

func windowsEnrollScript(loginServer string, authKey string, hostname string) string {
	hostnameLine := `$Hostname = $env:COMPUTERNAME`
	if strings.TrimSpace(hostname) != "" {
		hostnameLine = fmt.Sprintf("$Hostname = %q", hostname)
	}

	return fmt.Sprintf(`$ErrorActionPreference = "Stop"

$LoginServer = %q
$AuthKey = %q
%s

tailscale up --login-server $LoginServer --authkey $AuthKey --hostname $Hostname
tailscale status
`, loginServer, authKey, hostnameLine)
}

func stringOrDash(value *string) string {
	if value == nil || *value == "" {
		return "-"
	}
	return *value
}

func firstStringOrDash(values []string) string {
	if len(values) == 0 || values[0] == "" {
		return "-"
	}
	return values[0]
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return "-"
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
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

func envOrDefault(name string, fallback string) string {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	return value
}
