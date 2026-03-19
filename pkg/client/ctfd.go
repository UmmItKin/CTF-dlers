package client

import (
	"encoding/json"
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
	filesEndpoint       = "/files"
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
	client  *resty.Client
	baseURL string
	token   string
	config  *ClientConfig
}

type ClientConfig struct {
	BaseURL    string
	Token      string
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

	if config.Token == "" {
		return nil, fmt.Errorf("authentication token is required")
	}

	_, err := url.Parse(config.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	client := resty.New()
	client.SetBaseURL(strings.TrimSuffix(config.BaseURL, "/"))
	client.SetTimeout(config.Timeout)
	client.SetHeader(userAgentHeader, config.UserAgent)
	client.SetHeader(authorizationHeader, fmt.Sprintf("Bearer %s", config.Token))
	client.SetHeader(contentTypeHeader, contentTypeJSON)

	client.SetRetryCount(config.RetryCount)
	client.SetRetryWaitTime(config.RetryDelay)
	client.SetRetryMaxWaitTime(config.RetryDelay * 3)

	client.AddRetryCondition(func(r *resty.Response, err error) bool {
		if err != nil {
			return true
		}
		statusCode := r.StatusCode()
		return statusCode >= 500 || statusCode == http.StatusTooManyRequests
	})

	if config.Debug {
		client.SetDebug(true)
	}

	return &CTFdClient{
		client:  client,
		baseURL: config.BaseURL,
		token:   config.Token,
		config:  config,
	}, nil
}

func (c *CTFdClient) GetChallenges() ([]models.Challenge, error) {
	resp, err := c.client.R().
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

	resp, err := c.client.R().
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

func (c *CTFdClient) DownloadFile(fileURL string) ([]byte, error) {
	resp, err := c.client.R().Get(fileURL)

	if err != nil {
		return nil, fmt.Errorf("failed to download file from %s: %w", fileURL, err)
	}

	if err := c.checkResponseError(resp); err != nil {
		return nil, fmt.Errorf("API error while downloading file from %s: %w", fileURL, err)
	}

	return resp.Body(), nil
}

func (c *CTFdClient) DownloadFileToWriter(fileURL string, writer io.Writer) error {
	resp, err := c.client.R().
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

	resp, err := c.client.R().
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
	resp, err := c.client.R().
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

	if err := c.checkHTTPStatusCode(statusCode); err != nil {
		return err
	}

	if statusCode != http.StatusOK {
		var errorResp models.ErrorResponse
		if err := json.Unmarshal(resp.Body(), &errorResp); err == nil && !errorResp.Success {
			return fmt.Errorf("CTFd API error: %v", errorResp.Errors)
		}

		var simpleErrorResp models.SimpleErrorResponse
		if err := json.Unmarshal(resp.Body(), &simpleErrorResp); err == nil && !simpleErrorResp.Success {
			return fmt.Errorf("CTFd API error: %v", simpleErrorResp.Errors)
		}

		return fmt.Errorf("HTTP %d: %s", statusCode, string(resp.Body()))
	}

	return nil
}

func (c *CTFdClient) checkHTTPStatusCode(statusCode int) error {
	switch statusCode {
	case http.StatusOK:
		return nil
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

func (c *CTFdClient) GetBaseURL() string {
	return c.baseURL
}

func (c *CTFdClient) GetConfig() *ClientConfig {
	return c.config
}
