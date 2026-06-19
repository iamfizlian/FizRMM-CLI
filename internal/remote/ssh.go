package remote

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

type SSHConfig struct {
	User       string
	PrivateKey string
	KeyFile    string
	Port       string
	Timeout    time.Duration
}

type CommandResult struct {
	Stdout     string        `json:"stdout"`
	Stderr     string        `json:"stderr"`
	ExitCode   int           `json:"exit_code"`
	StartedAt  time.Time     `json:"started_at"`
	FinishedAt time.Time     `json:"finished_at"`
	Duration   time.Duration `json:"duration"`
}

func RunSSHCommand(ctx context.Context, cfg SSHConfig, host string, command string) (CommandResult, error) {
	started := time.Now().UTC()
	result := CommandResult{StartedAt: started, ExitCode: -1}

	host = strings.TrimSpace(host)
	command = strings.TrimSpace(command)
	if host == "" {
		return result, fmt.Errorf("host is required")
	}
	if command == "" {
		return result, fmt.Errorf("command is required")
	}
	if cfg.User == "" {
		cfg.User = "rmm"
	}
	if cfg.Port == "" {
		cfg.Port = "22"
	}
	if _, err := strconv.Atoi(cfg.Port); err != nil {
		return result, fmt.Errorf("invalid SSH port %q", cfg.Port)
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}

	signer, err := signerFromConfig(cfg)
	if err != nil {
		return result, err
	}

	sshConfig := &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         cfg.Timeout,
	}

	dialer := net.Dialer{Timeout: cfg.Timeout}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(host, cfg.Port))
	if err != nil {
		return result, fmt.Errorf("ssh dial %s: %w", host, err)
	}
	defer conn.Close()

	sshConn, chans, reqs, err := ssh.NewClientConn(conn, net.JoinHostPort(host, cfg.Port), sshConfig)
	if err != nil {
		return result, fmt.Errorf("ssh handshake %s: %w", host, err)
	}
	client := ssh.NewClient(sshConn, chans, reqs)
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return result, fmt.Errorf("ssh session: %w", err)
	}
	defer session.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	err = session.Run(command)
	result.FinishedAt = time.Now().UTC()
	result.Duration = result.FinishedAt.Sub(result.StartedAt)
	result.Stdout = stdout.String()
	result.Stderr = stderr.String()
	if err == nil {
		result.ExitCode = 0
		return result, nil
	}
	if exitErr, ok := err.(*ssh.ExitError); ok {
		result.ExitCode = exitErr.ExitStatus()
		return result, nil
	}
	return result, fmt.Errorf("ssh command failed: %w", err)
}

func signerFromConfig(cfg SSHConfig) (ssh.Signer, error) {
	key := strings.TrimSpace(cfg.PrivateKey)
	if key == "" && cfg.KeyFile != "" {
		content, err := os.ReadFile(cfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("read SSH private key: %w", err)
		}
		key = string(content)
	}
	if key == "" {
		return nil, fmt.Errorf("RMM_SSH_PRIVATE_KEY or RMM_SSH_PRIVATE_KEY_FILE is required")
	}
	signer, err := ssh.ParsePrivateKey([]byte(key))
	if err != nil {
		return nil, fmt.Errorf("parse SSH private key: %w", err)
	}
	return signer, nil
}
