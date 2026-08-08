package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"ctfd-downloader/pkg/models"

	"github.com/go-resty/resty/v2"
)

const (
	challengesEndpoint  = "/api/v1/challenges"
	authorizationHeader = "Authorization"
	userAgentHeader     = "User-Agent"
	contentTypeHeader   = "Content-Type"
	contentTypeJSON     = "application/json"
	defaultUserAgent    = "CTFd-Downloader/1.0"
	defaultTimeout      = 30 * time.Second
	maxRetryAttempts    = 3
	retryDelay          = 1 * time.Second
)

type CTFdClient struct {
	client   *resty.Client
	baseHost string
	token    string
	cookie   string
}

type ClientConfig struct {
	BaseURL    string
	Token      string
	Cookie     string // CTFd session cookie, alternative to Token
	Timeout    time.Duration
	UserAgent  string
	RateLimit  int
	RetryCount int
	RetryDelay time.Duration
	Debug      bool
}

func DefaultClientConfig() *ClientConfig {
	return &ClientConfig{
		Timeout:    defaultTimeout,
		UserAgent:  defaultUserAgent,
		RateLimit:  10,
		RetryCount: maxRetryAttempts,
		RetryDelay: retryDelay,
		Debug:      false,
	}
}

func NewCTFdClient(config *ClientConfig) (*CTFdClient, error) {
	if config == nil {
		config = DefaultClientConfig()
	}

	if config.BaseURL == "" {
		return nil, fmt.Errorf("base URL is required")
	}

	if config.Token == "" && config.Cookie == "" {
		return nil, fmt.Errorf("authentication is required: set a token or a session cookie")
	}

	parsed, err := url.ParseRequestURI(config.BaseURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("invalid base URL %q: must be an absolute http(s) URL", config.BaseURL)
	}

	client := resty.New()
	client.SetBaseURL(strings.TrimSuffix(config.BaseURL, "/"))
	client.SetTimeout(config.Timeout)
	client.SetHeader(userAgentHeader, config.UserAgent)
	client.SetHeader(contentTypeHeader, contentTypeJSON)
	// auth is attached per-request so the token never leaks to a foreign host

	client.SetRetryCount(config.RetryCount)
	client.SetRetryWaitTime(config.RetryDelay)
	client.SetRetryMaxWaitTime(config.RetryDelay * 3)

	client.AddRetryCondition(func(r *resty.Response, err error) bool {
		if err != nil {
			// don't retry cancellation
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return false
			}
			return true
		}
		statusCode := r.StatusCode()
		return statusCode >= 500 || statusCode == http.StatusTooManyRequests
	})

	if config.Debug {
		client.SetDebug(true)
	}

	return &CTFdClient{
		client:   client,
		baseHost: parsed.Host,
		token:    config.Token,
		cookie:   config.Cookie,
	}, nil
}

// auth attaches the token (preferred) or session cookie to a request.
func (c *CTFdClient) auth(r *resty.Request) *resty.Request {
	switch {
	case c.token != "":
		r.SetHeader(authorizationHeader, "Bearer "+c.token)
	case c.cookie != "":
		r.SetHeader("Cookie", cookieHeader(c.cookie))
	}
	return r
}

// cookieHeader accepts either a raw session value or a full "name=value" pair.
func cookieHeader(cookie string) string {
	if strings.Contains(cookie, "=") {
		return cookie
	}
	return "session=" + cookie
}

// apiReq returns an authenticated request for CTFd API calls.
func (c *CTFdClient) apiReq() *resty.Request {
	return c.auth(c.client.R())
}

// fileReq attaches auth only for same-host or relative download URLs.
func (c *CTFdClient) fileReq(fileURL string) *resty.Request {
	r := c.client.R()
	if u, err := url.Parse(fileURL); err == nil && (u.Host == "" || strings.EqualFold(u.Host, c.baseHost)) {
		c.auth(r)
	}
	return r
}

func (c *CTFdClient) GetChallenges() ([]models.Challenge, error) {
	resp, err := c.apiReq().
		SetResult(&models.ChallengeListResponse{}).
		Get(challengesEndpoint)

	if err != nil {
		return nil, fmt.Errorf("failed to fetch challenges: %w", err)
	}

	if err := c.checkResponseError(resp); err != nil {
		return nil, fmt.Errorf("API error while fetching challenges: %w", err)
	}

	result, ok := resp.Result().(*models.ChallengeListResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type")
	}

	if !result.Success {
		return nil, fmt.Errorf("API returned success=false")
	}

	return result.Data, nil
}

func (c *CTFdClient) GetChallenge(challengeID int) (*models.ChallengeDetailed, error) {
	endpoint := path.Join(challengesEndpoint, strconv.Itoa(challengeID))

	resp, err := c.apiReq().
		SetResult(&models.ChallengeDetailResponse{}).
		Get(endpoint)

	if err != nil {
		return nil, fmt.Errorf("failed to fetch challenge %d: %w", challengeID, err)
	}

	if err := c.checkResponseError(resp); err != nil {
		return nil, fmt.Errorf("API error while fetching challenge %d: %w", challengeID, err)
	}

	result, ok := resp.Result().(*models.ChallengeDetailResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type")
	}

	if !result.Success {
		return nil, fmt.Errorf("API returned success=false for challenge %d", challengeID)
	}

	return &result.Data, nil
}

func (c *CTFdClient) DownloadFileToWriter(fileURL string, writer io.Writer) error {
	resp, err := c.fileReq(fileURL).
		SetDoNotParseResponse(true).
		Get(fileURL)

	if err != nil {
		return fmt.Errorf("failed to download file from %s: %w", fileURL, err)
	}
	defer resp.RawBody().Close()

	if err := c.checkHTTPStatusCode(resp.StatusCode()); err != nil {
		return fmt.Errorf("API error while downloading file from %s: %w", fileURL, err)
	}

	_, err = io.Copy(writer, resp.RawBody())
	if err != nil {
		return fmt.Errorf("failed to write downloaded file: %w", err)
	}

	return nil
}

func (c *CTFdClient) GetChallengeSolves(challengeID int) ([]models.Solve, error) {
	endpoint := path.Join(challengesEndpoint, strconv.Itoa(challengeID), "solves")

	resp, err := c.apiReq().
		SetResult(&models.SolvesResponse{}).
		Get(endpoint)

	if err != nil {
		return nil, fmt.Errorf("failed to fetch solves for challenge %d: %w", challengeID, err)
	}

	if err := c.checkResponseError(resp); err != nil {
		return nil, fmt.Errorf("API error while fetching solves for challenge %d: %w", challengeID, err)
	}

	result, ok := resp.Result().(*models.SolvesResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type")
	}

	if !result.Success {
		return nil, fmt.Errorf("API returned success=false for challenge %d solves", challengeID)
	}

	return result.Data, nil
}

func (c *CTFdClient) TestConnection() error {
	resp, err := c.apiReq().
		SetResult(&models.ChallengeListResponse{}).
		SetQueryParam("limit", "1").
		Get(challengesEndpoint)

	if err != nil {
		return fmt.Errorf("connection test failed: %w", err)
	}

	if err := c.checkResponseError(resp); err != nil {
		return fmt.Errorf("authentication test failed: %w", err)
	}

	return nil
}

func (c *CTFdClient) checkResponseError(resp *resty.Response) error {
	statusCode := resp.StatusCode()
	if statusCode >= 200 && statusCode < 300 {
		return nil
	}

	// surface the server's error message if present
	body := resp.Body()
	var errorResp models.ErrorResponse
	if err := json.Unmarshal(body, &errorResp); err == nil && len(errorResp.Errors) > 0 {
		return fmt.Errorf("%w: %v", c.checkHTTPStatusCode(statusCode), errorResp.Errors)
	}
	var simpleErrorResp models.SimpleErrorResponse
	if err := json.Unmarshal(body, &simpleErrorResp); err == nil && len(simpleErrorResp.Errors) > 0 {
		return fmt.Errorf("%w: %v", c.checkHTTPStatusCode(statusCode), simpleErrorResp.Errors)
	}

	return c.checkHTTPStatusCode(statusCode)
}

func (c *CTFdClient) checkHTTPStatusCode(statusCode int) error {
	if statusCode >= 200 && statusCode < 300 {
		return nil
	}
	switch statusCode {
	case http.StatusUnauthorized:
		return fmt.Errorf("authentication failed (401): check your token")
	case http.StatusForbidden:
		return fmt.Errorf("access forbidden (403): insufficient permissions or CTF not started")
	case http.StatusNotFound:
		return fmt.Errorf("resource not found (404)")
	case http.StatusTooManyRequests:
		return fmt.Errorf("rate limited (429): too many requests")
	case http.StatusInternalServerError:
		return fmt.Errorf("server error (500): internal server error")
	case http.StatusBadGateway:
		return fmt.Errorf("server error (502): bad gateway")
	case http.StatusServiceUnavailable:
		return fmt.Errorf("server error (503): service unavailable")
	case http.StatusGatewayTimeout:
		return fmt.Errorf("server error (504): gateway timeout")
	default:
		if statusCode >= 400 && statusCode < 500 {
			return fmt.Errorf("client error (%d)", statusCode)
		}
		if statusCode >= 500 {
			return fmt.Errorf("server error (%d)", statusCode)
		}
		return fmt.Errorf("unexpected status code: %d", statusCode)
	}
}
