package headscale

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

type Node struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	GivenName   string          `json:"givenName"`
	User        User            `json:"user"`
	IPAddresses []string        `json:"ipAddresses"`
	Online      bool            `json:"online"`
	LastSeen    *time.Time      `json:"lastSeen,omitempty"`
	Expiry      *time.Time      `json:"expiry,omitempty"`
	ForcedTags  []string        `json:"forcedTags,omitempty"`
	InvalidTags []string        `json:"invalidTags,omitempty"`
	ValidTags   []string        `json:"validTags,omitempty"`
	Raw         json.RawMessage `json:"-"`
}

type User struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type PreAuthKey struct {
	User       string     `json:"user"`
	ID         string     `json:"id"`
	Key        string     `json:"key"`
	Reusable   bool       `json:"reusable"`
	Ephemeral  bool       `json:"ephemeral"`
	Used       bool       `json:"used"`
	Expiration *time.Time `json:"expiration,omitempty"`
	CreatedAt  *time.Time `json:"createdAt,omitempty"`
	ACLTags    []string   `json:"aclTags"`
}

type CreatePreAuthKeyRequest struct {
	User       string    `json:"user"`
	Reusable   bool      `json:"reusable"`
	Ephemeral  bool      `json:"ephemeral"`
	Expiration time.Time `json:"expiration"`
	ACLTags    []string  `json:"aclTags,omitempty"`
}

func New(baseURL string, apiKey string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) Configured() bool {
	return c != nil && c.baseURL != "" && c.apiKey != ""
}

func (c *Client) ListNodes(ctx context.Context) ([]Node, error) {
	var response struct {
		Nodes []json.RawMessage `json:"nodes"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/v1/node", nil, &response); err != nil {
		return nil, err
	}

	nodes := make([]Node, 0, len(response.Nodes))
	for _, raw := range response.Nodes {
		var node Node
		if err := json.Unmarshal(raw, &node); err != nil {
			return nil, fmt.Errorf("decode headscale node: %w", err)
		}
		node.Raw = raw
		nodes = append(nodes, node)
	}

	return nodes, nil
}

func (c *Client) CreatePreAuthKey(ctx context.Context, request CreatePreAuthKeyRequest) (PreAuthKey, error) {
	var response struct {
		PreAuthKey PreAuthKey `json:"preAuthKey"`
	}
	if err := c.do(ctx, http.MethodPost, "/api/v1/preauthkey", request, &response); err != nil {
		return PreAuthKey{}, err
	}
	return response.PreAuthKey, nil
}

func (c *Client) do(ctx context.Context, method string, path string, requestBody any, responseBody any) error {
	if !c.Configured() {
		return fmt.Errorf("headscale client is not configured")
	}

	var body *bytes.Reader
	if requestBody == nil {
		body = bytes.NewReader(nil)
	} else {
		encoded, err := json.Marshal(requestBody)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	if requestBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("headscale returned %s", resp.Status)
	}

	if responseBody == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(responseBody); err != nil {
		return err
	}
	return nil
}
