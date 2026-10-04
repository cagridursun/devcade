// Package leaderboard implements an optional anonymous community score service.
// Scores are client reported; these rankings are not cheat-resistant competition.
package leaderboard

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/cagridursun/devcade/internal/profile"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Row struct {
	Rank     int    `json:"rank"`
	PlayerID string `json:"player_id"`
	Username string `json:"username"`
	Score    int    `json:"score"`
}
type Board struct {
	Rows []Row `json:"rows"`
	Own  *Row  `json:"own,omitempty"`
}
type Registration struct {
	ID    string `json:"id"`
	Token string `json:"token"`
}
type Client struct {
	Endpoint string
	http     *http.Client
}

var ErrNameTaken = errors.New("username already taken")
var ErrIdentity = errors.New("player identity rejected")

func NewClient(endpoint string) (*Client, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1"))) {
		return nil, fmt.Errorf("leaderboard requires HTTPS (HTTP only on loopback)")
	}
	return &Client{Endpoint: strings.TrimRight(endpoint, "/"), http: &http.Client{Timeout: 3 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func (c *Client) request(ctx context.Context, method, path, token string, in, out any) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.Endpoint+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusConflict {
		return ErrNameTaken
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return ErrIdentity
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("leaderboard HTTP %d", resp.StatusCode)
	}
	if out == nil {
		return nil
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 32769))
	if err != nil {
		return err
	}
	if len(b) > 32768 {
		return fmt.Errorf("leaderboard response too large")
	}
	return json.Unmarshal(b, out)
}
func (c *Client) Register(ctx context.Context, name string) (Registration, error) {
	var r Registration
	err := c.request(ctx, "POST", "/v1/players", "", struct {
		Username string `json:"username"`
	}{name}, &r)
	if err == nil && (len(r.ID) != 32 || len(r.Token) != 64) {
		err = fmt.Errorf("invalid identity response")
	}
	return r, err
}
func (c *Client) Submit(ctx context.Context, token, game string, n int) error {
	return c.request(ctx, "PUT", "/v1/best", token, struct {
		Game  string `json:"game"`
		Score int    `json:"score"`
	}{game, n}, nil)
}
func (c *Client) Fetch(ctx context.Context, game, id string) (Board, error) {
	var b Board
	err := c.request(ctx, "GET", "/v1/leaderboards/"+game+"?player="+url.QueryEscape(id), "", nil, &b)
	if err == nil {
		if len(b.Rows) > 20 {
			err = fmt.Errorf("invalid leaderboard")
		}
		for _, r := range b.Rows {
			if !profile.ValidUsername(r.Username) || !profile.ValidScore(game, r.Score) || r.Rank < 1 {
				err = fmt.Errorf("invalid leaderboard row")
			}
		}
		if b.Own != nil && (!profile.ValidUsername(b.Own.Username) || !profile.ValidScore(game, b.Own.Score) || b.Own.Rank < 1 || b.Own.PlayerID != id) {
			err = fmt.Errorf("invalid own row")
		}
	}
	return b, err
}
