package yousee

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"time"
)

const (
	graphqlEndpoint = "https://graphql-1458.api.247e.com/graphql"
	imageSize       = 512
	pageSize        = 50
	maxPages        = 20
)

// Client is a YouSee Musik GraphQL client.
type Client struct {
	http *http.Client
	auth *authManager
}

// New creates a YouSee client authenticated with the given credentials.
func New(username, password string) *Client {
	jar, _ := cookiejar.New(nil)
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Jar:     jar,
	}
	return &Client{
		http: httpClient,
		auth: newAuthManager(username, password, httpClient),
	}
}

// Ping verifies that authentication succeeds.
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.auth.token(ctx)
	return err
}

// gqlResponse is a generic GraphQL response envelope.
type gqlResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// post executes a GraphQL query and unmarshals the "data" field into out.
func (c *Client) post(ctx context.Context, query string, variables map[string]any, out any) error {
	token, err := c.auth.token(ctx)
	if err != nil {
		return err
	}

	payload, err := json.Marshal(map[string]any{"query": query, "variables": variables})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, graphqlEndpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept-Language", "da")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		c.auth.invalidate()
		return fmt.Errorf("authentication with YouSee failed")
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("yousee graphql http %d", resp.StatusCode)
	}

	var envelope gqlResponse
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return err
	}
	if len(envelope.Errors) > 0 {
		return fmt.Errorf("yousee graphql error: %s", envelope.Errors[0].Message)
	}
	if out != nil {
		return json.Unmarshal(envelope.Data, out)
	}
	return nil
}

// --- small IO helpers shared with auth.go ---

func decodeJSON(r io.Reader, out any) error {
	return json.NewDecoder(r).Decode(out)
}

func readAll(r io.Reader) ([]byte, error) {
	return io.ReadAll(r)
}
