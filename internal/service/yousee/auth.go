// Package yousee implements a client for the YouSee Musik GraphQL service,
// including its username/password authentication flow.
package yousee

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	tokenURL          = "https://musik.yousee.dk/api/token"
	delegatedLoginURL = "https://musik.yousee.dk/api/delegatedlogin"
	loginHost         = "https://login.yousee.dk"
)

var (
	reAction       = regexp.MustCompile(`action="([^"]+)"`)
	reAccessToken  = regexp.MustCompile(`localStorage\.setItem\("accesstoken", "([^"]+)"`)
	reRefreshToken = regexp.MustCompile(`localStorage\.setItem\("refreshtoken", "([^"]+)"`)
)

// accessToken wraps a raw YouSee access token and exposes its expiry.
type accessToken struct {
	raw       string
	expiresOn int64
}

func parseAccessToken(raw string) *accessToken {
	t := &accessToken{raw: raw}
	for _, part := range strings.Split(raw, "&") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		if kv[0] == "ExpiresOn" {
			if v, err := strconv.ParseInt(kv[1], 10, 64); err == nil {
				t.expiresOn = v
			}
		}
	}
	return t
}

func (t *accessToken) expired() bool {
	return t.expiresOn == 0 || t.expiresOn <= time.Now().Unix()
}

// authManager handles authentication against YouSee Musik.
type authManager struct {
	username string
	password string
	client   *http.Client

	mu           sync.Mutex
	access       *accessToken
	refreshToken string
}

func newAuthManager(username, password string, client *http.Client) *authManager {
	return &authManager{username: username, password: password, client: client}
}

// invalidate clears the cached access token.
func (a *authManager) invalidate() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.access = nil
}

// token returns a valid access token, refreshing or logging in as needed.
func (a *authManager) token(ctx context.Context) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.access != nil && !a.access.expired() {
		return a.access.raw, nil
	}

	if a.refreshToken != "" {
		if tok, err := a.refresh(ctx); err == nil && tok != "" {
			return tok, nil
		}
	}

	return a.login(ctx)
}

func (a *authManager) refresh(ctx context.Context) (string, error) {
	form := url.Values{"refresh_token": {a.refreshToken}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := a.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Status      int `json:"status"`
		TokenResult struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		} `json:"tokenResult"`
	}
	if err := decodeJSON(resp.Body, &result); err != nil {
		return "", err
	}
	if result.Status != 0 || result.TokenResult.AccessToken == "" {
		return "", fmt.Errorf("refresh token flow failed")
	}

	a.access = parseAccessToken(result.TokenResult.AccessToken)
	a.refreshToken = result.TokenResult.RefreshToken
	return a.access.raw, nil
}

func (a *authManager) login(ctx context.Context) (string, error) {
	// Step 1: fetch the delegated login form to obtain the post action + cookies.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, delegatedLoginURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return "", err
	}
	body, err := readAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return "", err
	}

	m := reAction.FindSubmatch(body)
	if m == nil {
		return "", fmt.Errorf("login form action not found")
	}
	action := string(m[1])

	// Step 2: submit credentials.
	form := url.Values{
		"pf.username":  {a.username},
		"pf.pass":      {a.password},
		"pf.ok":        {"clicked"},
		"pf.adapterId": {"MusicUsernamePasswordAdapter"},
	}
	loginReq, err := http.NewRequestWithContext(ctx, http.MethodPost, loginHost+action, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	loginResp, err := a.client.Do(loginReq)
	if err != nil {
		return "", err
	}
	loginBody, err := readAll(loginResp.Body)
	loginResp.Body.Close()
	if err != nil {
		return "", err
	}

	at := reAccessToken.FindSubmatch(loginBody)
	rt := reRefreshToken.FindSubmatch(loginBody)
	if at == nil || rt == nil {
		return "", fmt.Errorf("authentication with YouSee failed (check credentials)")
	}

	a.access = parseAccessToken(string(at[1]))
	a.refreshToken = string(rt[1])
	return a.access.raw, nil
}
