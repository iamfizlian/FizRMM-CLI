package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"fizrmm-cli/internal/enrollment"
	"fizrmm-cli/internal/version"
)

type command struct {
	Name        string
	Description string
}

var commands = []command{
	{Name: "tenant list", Description: "List tenants"},
	{Name: "tui", Description: "Open terminal operator UI"},
	{Name: "site list", Description: "List sites"},
	{Name: "node list", Description: "List managed nodes"},
	{Name: "node check", Description: "Run a built-in node diagnostic"},
	{Name: "check list", Description: "List built-in diagnostics"},
	{Name: "node enroll-script", Description: "Generate an endpoint enrollment script"},
	{Name: "node ssh", Description: "Open an audited SSH session"},
	{Name: "exec", Description: "Run an audited command over SSH"},
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
	case commandName == "check list":
		listChecks(*jsonOutput)
	case len(args) >= 2 && args[0] == "node" && args[1] == "check":
		if err := runCheck(*apiURL, args[2:], *jsonOutput); err != nil {
			fmt.Fprintf(os.Stderr, "node check failed: %v\n", err)
			os.Exit(1)
		}
	case commandName == "tui":
		if err := runTUI(*apiURL); err != nil {
			fmt.Fprintf(os.Stderr, "tui failed: %v\n", err)
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
	case args[0] == "exec":
		if err := runCommand(*apiURL, args[1:], *jsonOutput); err != nil {
			fmt.Fprintf(os.Stderr, "exec failed: %v\n", err)
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

type commandRunResponse struct {
	Data commandRunResult `json:"data"`
	Meta any              `json:"meta"`
}

type commandRunResult struct {
	NodeID     int64     `json:"node_id"`
	Hostname   string    `json:"hostname"`
	Command    string    `json:"command"`
	Stdout     string    `json:"stdout"`
	Stderr     string    `json:"stderr"`
	ExitCode   int       `json:"exit_code"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
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

type builtInCheck struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Command     string `json:"command"`
}

var builtInChecks = []builtInCheck{
	{
		Name:        "summary",
		Description: "Hostname, OS, kernel, uptime, load, memory, and disk summary",
		Command:     `printf 'Hostname: '; hostname; printf 'OS: '; . /etc/os-release 2>/dev/null && echo "$PRETTY_NAME" || uname -s; printf 'Kernel: '; uname -r; printf 'Uptime: '; uptime -p 2>/dev/null || uptime; printf '\nLoad:\n'; cat /proc/loadavg; printf '\nMemory:\n'; free -h; printf '\nDisk:\n'; df -hT -x tmpfs -x devtmpfs`,
	},
	{
		Name:        "disk",
		Description: "Filesystem usage excluding tmpfs/devtmpfs",
		Command:     `df -hT -x tmpfs -x devtmpfs`,
	},
	{
		Name:        "memory",
		Description: "Memory and swap usage",
		Command:     `free -h && printf '\nTop memory processes:\n' && ps -eo pid,comm,%mem,%cpu --sort=-%mem | head -n 11`,
	},
	{
		Name:        "load",
		Description: "Load average and top CPU processes",
		Command:     `uptime; printf '\nTop CPU processes:\n'; ps -eo pid,comm,%cpu,%mem --sort=-%cpu | head -n 11`,
	},
	{
		Name:        "services",
		Description: "Failed systemd services",
		Command:     `systemctl --failed --no-pager || true`,
	},
	{
		Name:        "updates",
		Description: "Pending OS package updates where supported",
		Command:     `if command -v apt-get >/dev/null 2>&1; then apt list --upgradable 2>/dev/null | sed -n '1,40p'; elif command -v dnf >/dev/null 2>&1; then dnf check-update || true; elif command -v yum >/dev/null 2>&1; then yum check-update || true; elif command -v apk >/dev/null 2>&1; then apk version -l '<'; else echo 'No supported package manager found'; fi`,
	},
	{
		Name:        "ports",
		Description: "Listening TCP/UDP ports",
		Command:     `ss -tulpen 2>/dev/null || netstat -tulpen 2>/dev/null || echo 'ss/netstat not found'`,
	},
	{
		Name:        "tailscale",
		Description: "Tailscale status and assigned IPs",
		Command:     `tailscale ip; printf '\n'; tailscale status`,
	},
}

func listNodes(apiURL string, jsonOutput bool) error {
	nodes, body, err := fetchNodes(apiURL)
	if err != nil {
		return err
	}
	if jsonOutput {
		fmt.Fprintln(os.Stdout, string(body))
		return nil
	}

	printNodes(nodes)
	return nil
}

func fetchNodes(apiURL string) ([]node, []byte, error) {
	url := strings.TrimRight(apiURL, "/") + "/v1/nodes"
	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Get(url)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, body, fmt.Errorf("api returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var decoded nodeListResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, body, err
	}
	return decoded.Data, body, nil
}

func printNodes(nodes []node) {
	if len(nodes) == 0 {
		fmt.Fprintln(os.Stdout, "No nodes found.")
		return
	}

	fmt.Fprintf(os.Stdout, "%-6s %-24s %-12s %-16s %-20s\n", "ID", "HOSTNAME", "STATUS", "TENANT", "TAILNET IP")
	for _, node := range nodes {
		fmt.Fprintf(os.Stdout, "%-6d %-24s %-12s %-16d %-20s\n",
			node.ID,
			node.Hostname,
			node.Status,
			node.TenantID,
			stringOrDash(node.TailnetIP),
		)
	}
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

	script := enrollment.LinuxScript(*loginServer, key.Key, *hostname, "", "")
	if normalizedOS == "windows" {
		script = enrollment.WindowsScript(*loginServer, key.Key, *hostname)
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

func runCommand(apiURL string, args []string, jsonOutput bool) error {
	flags := flag.NewFlagSet("exec", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	node := flags.String("node", "", "node id or hostname")
	timeout := flags.String("timeout", "30s", "command timeout")
	if err := flags.Parse(args); err != nil {
		return err
	}
	command := strings.TrimSpace(strings.Join(flags.Args(), " "))
	if *node == "" {
		return fmt.Errorf("--node is required")
	}
	if command == "" {
		return fmt.Errorf("command is required")
	}
	return runRemoteCommand(apiURL, *node, command, *timeout, jsonOutput)
}

func listChecks(jsonOutput bool) {
	if jsonOutput {
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
			"data": builtInChecks,
		})
		return
	}
	fmt.Fprintf(os.Stdout, "%-14s %s\n", "CHECK", "DESCRIPTION")
	for _, check := range builtInChecks {
		fmt.Fprintf(os.Stdout, "%-14s %s\n", check.Name, check.Description)
	}
}

func runCheck(apiURL string, args []string, jsonOutput bool) error {
	flags := flag.NewFlagSet("node check", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	node := flags.String("node", "", "node id or hostname")
	timeout := flags.String("timeout", "45s", "check timeout")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *node == "" {
		return fmt.Errorf("--node is required")
	}
	if flags.NArg() != 1 {
		return fmt.Errorf("check name is required")
	}
	check, ok := findCheck(flags.Arg(0))
	if !ok {
		return fmt.Errorf("unknown check %q", flags.Arg(0))
	}
	if !jsonOutput {
		fmt.Fprintf(os.Stdout, "== %s on %s ==\n", check.Name, *node)
	}
	return runRemoteCommand(apiURL, *node, check.Command, *timeout, jsonOutput)
}

func runRemoteCommand(apiURL string, node string, command string, timeout string, jsonOutput bool) error {
	body, err := apiRequest(apiURL, http.MethodPost, "/v1/commands/run", map[string]any{
		"node":    node,
		"command": command,
		"timeout": timeout,
	})
	if err != nil {
		return err
	}
	if jsonOutput {
		fmt.Fprintln(os.Stdout, string(body))
		return nil
	}

	var decoded commandRunResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return err
	}
	if decoded.Data.Stdout != "" {
		fmt.Fprint(os.Stdout, decoded.Data.Stdout)
		if !strings.HasSuffix(decoded.Data.Stdout, "\n") {
			fmt.Fprintln(os.Stdout)
		}
	}
	if decoded.Data.Stderr != "" {
		fmt.Fprint(os.Stderr, decoded.Data.Stderr)
		if !strings.HasSuffix(decoded.Data.Stderr, "\n") {
			fmt.Fprintln(os.Stderr)
		}
	}
	if decoded.Data.ExitCode != 0 {
		return fmt.Errorf("remote command exited %d", decoded.Data.ExitCode)
	}
	return nil
}

func findCheck(name string) (builtInCheck, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, check := range builtInChecks {
		if check.Name == name {
			return check, true
		}
	}
	return builtInCheck{}, false
}

func runTUI(apiURL string) error {
	reader := bufio.NewReader(os.Stdin)
	for {
		clearScreen()
		fmt.Fprintln(os.Stdout, "FizRMM")
		fmt.Fprintln(os.Stdout, strings.Repeat("=", 72))
		nodes, _, err := fetchNodes(apiURL)
		if err != nil {
			fmt.Fprintf(os.Stdout, "Node refresh failed: %v\n\n", err)
		} else {
			printNodes(nodes)
		}
		fmt.Fprintln(os.Stdout)
		fmt.Fprintln(os.Stdout, "Actions:")
		fmt.Fprintln(os.Stdout, "  1. Run built-in check")
		fmt.Fprintln(os.Stdout, "  2. Run custom command")
		fmt.Fprintln(os.Stdout, "  3. Refresh")
		fmt.Fprintln(os.Stdout, "  q. Quit")
		fmt.Fprint(os.Stdout, "\nSelect: ")

		choice, err := readLine(reader)
		if err != nil {
			return err
		}
		switch strings.ToLower(choice) {
		case "1":
			if err := tuiRunCheck(apiURL, reader, nodes); err != nil {
				return err
			}
		case "2":
			if err := tuiRunCustomCommand(apiURL, reader, nodes); err != nil {
				return err
			}
		case "3", "":
			continue
		case "q", "quit", "exit":
			return nil
		default:
			pause(reader, "Unknown action.")
		}
	}
}

func tuiRunCheck(apiURL string, reader *bufio.Reader, nodes []node) error {
	selected, ok, err := promptNode(reader, nodes)
	if err != nil || !ok {
		return err
	}
	clearScreen()
	fmt.Fprintf(os.Stdout, "Checks for %s\n", selected.Hostname)
	fmt.Fprintln(os.Stdout, strings.Repeat("=", 72))
	for i, check := range builtInChecks {
		fmt.Fprintf(os.Stdout, "%2d. %-12s %s\n", i+1, check.Name, check.Description)
	}
	fmt.Fprint(os.Stdout, "\nCheck: ")
	choice, err := readLine(reader)
	if err != nil {
		return err
	}
	index, ok := parseSelection(choice, len(builtInChecks))
	if !ok {
		pause(reader, "Invalid check.")
		return nil
	}
	check := builtInChecks[index]
	clearScreen()
	fmt.Fprintf(os.Stdout, "Running %s on %s\n%s\n", check.Name, selected.Hostname, strings.Repeat("=", 72))
	err = runRemoteCommand(apiURL, selected.Hostname, check.Command, "45s", false)
	if err != nil {
		fmt.Fprintf(os.Stdout, "\nError: %v\n", err)
	}
	pause(reader, "")
	return nil
}

func tuiRunCustomCommand(apiURL string, reader *bufio.Reader, nodes []node) error {
	selected, ok, err := promptNode(reader, nodes)
	if err != nil || !ok {
		return err
	}
	fmt.Fprintf(os.Stdout, "Command for %s: ", selected.Hostname)
	command, err := readLine(reader)
	if err != nil {
		return err
	}
	if strings.TrimSpace(command) == "" {
		return nil
	}
	clearScreen()
	fmt.Fprintf(os.Stdout, "Running command on %s\n%s\n", selected.Hostname, strings.Repeat("=", 72))
	err = runRemoteCommand(apiURL, selected.Hostname, command, "45s", false)
	if err != nil {
		fmt.Fprintf(os.Stdout, "\nError: %v\n", err)
	}
	pause(reader, "")
	return nil
}

func promptNode(reader *bufio.Reader, nodes []node) (node, bool, error) {
	if len(nodes) == 0 {
		pause(reader, "No nodes available.")
		return node{}, false, nil
	}
	fmt.Fprintln(os.Stdout)
	fmt.Fprint(os.Stdout, "Node number/id/hostname: ")
	choice, err := readLine(reader)
	if err != nil {
		return node{}, false, err
	}
	choice = strings.TrimSpace(choice)
	if choice == "" {
		return node{}, false, nil
	}
	if index, ok := parseSelection(choice, len(nodes)); ok {
		return nodes[index], true, nil
	}
	for _, node := range nodes {
		if fmt.Sprint(node.ID) == choice || strings.EqualFold(node.Hostname, choice) {
			return node, true, nil
		}
	}
	pause(reader, "Node not found.")
	return node{}, false, nil
}

func parseSelection(value string, max int) (int, bool) {
	var selected int
	if _, err := fmt.Sscanf(strings.TrimSpace(value), "%d", &selected); err != nil {
		return 0, false
	}
	if selected < 1 || selected > max {
		return 0, false
	}
	return selected - 1, true
}

func readLine(reader *bufio.Reader) (string, error) {
	value, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimSpace(value), nil
}

func pause(reader *bufio.Reader, message string) {
	if message != "" {
		fmt.Fprintln(os.Stdout, message)
	}
	fmt.Fprint(os.Stdout, "\nPress Enter to continue...")
	_, _ = reader.ReadString('\n')
}

func clearScreen() {
	fmt.Fprint(os.Stdout, "\033[H\033[2J")
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
