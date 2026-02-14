package autotrolej

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultTimeout       = 15 * time.Second
	defaultTokenTTL      = 60 * time.Minute
	defaultRefreshMargin = 5 * time.Minute
)

type Config struct {
	BaseURL       string
	Username      string
	Password      string
	Timeout       time.Duration
	TokenTTL      time.Duration
	RefreshMargin time.Duration
}

type Client struct {
	httpClient    *http.Client
	baseURL       string
	username      string
	password      string
	token         string
	tokenAcquired time.Time
	tokenTTL      time.Duration
	refreshMargin time.Duration
	now           func() time.Time
}

type LiveBus struct {
	GBR         *int     `json:"gbr"`
	Lon         *float64 `json:"lon"`
	Lat         *float64 `json:"lat"`
	VoznjaID    *int     `json:"voznjaId"`
	VoznjaBusID *int     `json:"voznjaBusId"`
}

type AutobusiResponse struct {
	Msg string    `json:"msg"`
	Res []LiveBus `json:"res"`
	Err bool      `json:"err"`
}

type tokenEnvelope struct {
	Msg string `json:"msg"`
	Res string `json:"res"`
	Err bool   `json:"err"`
}

type httpStatusError struct {
	statusCode int
	body       string
}

func (e *httpStatusError) Error() string {
	if e.body == "" {
		return fmt.Sprintf("http status %d", e.statusCode)
	}
	return fmt.Sprintf("http status %d: %s", e.statusCode, e.body)
}

func (e *httpStatusError) StatusCode() int {
	return e.statusCode
}

func NewClient(cfg Config) (*Client, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		return nil, errors.New("base URL is required")
	}
	if _, err := url.ParseRequestURI(baseURL); err != nil {
		return nil, fmt.Errorf("invalid base URL %q: %w", baseURL, err)
	}

	username := strings.TrimSpace(cfg.Username)
	if username == "" {
		return nil, errors.New("username is required")
	}

	password := strings.TrimSpace(cfg.Password)
	if password == "" {
		return nil, errors.New("password is required")
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	tokenTTL := cfg.TokenTTL
	if tokenTTL <= 0 {
		tokenTTL = defaultTokenTTL
	}

	refreshMargin := cfg.RefreshMargin
	if refreshMargin <= 0 {
		refreshMargin = defaultRefreshMargin
	}

	if refreshMargin >= tokenTTL {
		return nil, fmt.Errorf("refresh margin (%s) must be smaller than token TTL (%s)", refreshMargin, tokenTTL)
	}

	return &Client{
		httpClient:    &http.Client{Timeout: timeout},
		baseURL:       baseURL,
		username:      username,
		password:      password,
		tokenTTL:      tokenTTL,
		refreshMargin: refreshMargin,
		now:           time.Now,
	}, nil
}

func (c *Client) Login(ctx context.Context) error {
	body, err := c.doRequest(ctx, http.MethodGet, "/api/open/v1/token/login", map[string]string{
		"Username": c.username,
		"Password": c.password,
	})
	if err != nil {
		return fmt.Errorf("login request failed: %w", err)
	}

	token, err := parseToken(body)
	if err != nil {
		return fmt.Errorf("parse login token: %w", err)
	}

	c.token = token
	c.tokenAcquired = c.now().UTC()
	return nil
}

func (c *Client) RefreshToken(ctx context.Context) error {
	if strings.TrimSpace(c.token) == "" {
		return errors.New("cannot refresh: token is empty")
	}

	body, err := c.doRequest(ctx, http.MethodGet, "/api/open/v1/token/refresh", map[string]string{
		"Username": c.username,
		"token":    c.token,
	})
	if err != nil {
		return fmt.Errorf("refresh request failed: %w", err)
	}

	token, err := parseToken(body)
	if err != nil {
		return fmt.Errorf("parse refresh token: %w", err)
	}

	c.token = token
	c.tokenAcquired = c.now().UTC()
	return nil
}

func (c *Client) GetAutobusi(ctx context.Context) (AutobusiResponse, error) {
	if err := c.ensureToken(ctx); err != nil {
		return AutobusiResponse{}, err
	}

	response, err := c.fetchAutobusi(ctx)
	if err == nil {
		return response, nil
	}

	if !isAuthError(err) {
		return AutobusiResponse{}, err
	}

	if refreshErr := c.RefreshToken(ctx); refreshErr != nil {
		if loginErr := c.Login(ctx); loginErr != nil {
			return AutobusiResponse{}, fmt.Errorf("autobusi request failed (%v); refresh/login recovery failed (%v / %v)", err, refreshErr, loginErr)
		}
	}

	response, retryErr := c.fetchAutobusi(ctx)
	if retryErr != nil {
		return AutobusiResponse{}, retryErr
	}

	return response, nil
}

func (c *Client) ensureToken(ctx context.Context) error {
	if strings.TrimSpace(c.token) == "" {
		if err := c.Login(ctx); err != nil {
			return fmt.Errorf("login: %w", err)
		}
		return nil
	}

	refreshAfter := c.tokenTTL - c.refreshMargin
	if refreshAfter <= 0 {
		refreshAfter = c.tokenTTL
	}

	if c.now().UTC().Sub(c.tokenAcquired) >= refreshAfter {
		if err := c.RefreshToken(ctx); err != nil {
			if loginErr := c.Login(ctx); loginErr != nil {
				return fmt.Errorf("refresh failed (%v), login fallback failed (%v)", err, loginErr)
			}
		}
	}

	return nil
}

func (c *Client) fetchAutobusi(ctx context.Context) (AutobusiResponse, error) {
	body, err := c.doRequest(ctx, http.MethodGet, "/api/open/v1/voznired/autobusi", map[string]string{
		"token": c.token,
	})
	if err != nil {
		return AutobusiResponse{}, fmt.Errorf("request /autobusi: %w", err)
	}

	var response AutobusiResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return AutobusiResponse{}, fmt.Errorf("decode autobusi response: %w", err)
	}

	if response.Err {
		if isAuthMessage(response.Msg) {
			return AutobusiResponse{}, fmt.Errorf("autobusi reported auth error: %s", response.Msg)
		}
		return AutobusiResponse{}, fmt.Errorf("autobusi reported error: %s", response.Msg)
	}

	return response, nil
}

func (c *Client) doRequest(ctx context.Context, method, path string, headers map[string]string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	for key, value := range headers {
		if strings.TrimSpace(value) == "" {
			continue
		}
		req.Header.Set(key, value)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, &httpStatusError{statusCode: resp.StatusCode, body: strings.TrimSpace(string(body))}
	}

	return body, nil
}

func parseToken(body []byte) (string, error) {
	var envelope tokenEnvelope
	if err := json.Unmarshal(body, &envelope); err == nil {
		if envelope.Err {
			return "", fmt.Errorf("token endpoint returned error: %s", envelope.Msg)
		}
		if token := strings.TrimSpace(envelope.Res); token != "" {
			return token, nil
		}
	}

	var rawToken string
	if err := json.Unmarshal(body, &rawToken); err == nil {
		if token := strings.TrimSpace(rawToken); token != "" {
			return token, nil
		}
	}

	var objectToken struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &objectToken); err == nil {
		if token := strings.TrimSpace(objectToken.Token); token != "" {
			return token, nil
		}
	}

	plain := strings.TrimSpace(string(body))
	plain = strings.Trim(plain, `"'`)
	if plain != "" && !strings.HasPrefix(plain, "{") && !strings.HasPrefix(plain, "[") {
		return plain, nil
	}

	return "", fmt.Errorf("token not found in response: %s", strings.TrimSpace(string(body)))
}

func isAuthError(err error) bool {
	var statusErr *httpStatusError
	if errors.As(err, &statusErr) {
		return statusErr.StatusCode() == http.StatusUnauthorized || statusErr.StatusCode() == http.StatusForbidden
	}

	return isAuthMessage(err.Error())
}

func isAuthMessage(message string) bool {
	normalized := strings.ToLower(strings.TrimSpace(message))
	if normalized == "" {
		return false
	}

	keywords := []string{"token", "auth", "unauthor", "forbidden", "istek", "neva"}
	for _, keyword := range keywords {
		if strings.Contains(normalized, keyword) {
			return true
		}
	}

	return false
}
