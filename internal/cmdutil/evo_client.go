package cmdutil

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/evopayment/evo-cli/internal/core"
	"github.com/evopayment/evo-cli/internal/output"
)

// Response holds the raw API response from DoAPI.
type Response struct {
	StatusCode int
	Body       map[string]interface{}
}

// EvoClient wraps an HTTP client and config to provide Evo Payment API access.
type EvoClient struct {
	httpClient *http.Client
	config     *core.CliConfig
	ioStreams  *IOStreams
}

// NewEvoClient creates a new EvoClient.
func NewEvoClient(httpClient *http.Client, config *core.CliConfig, ioStreams *IOStreams) *EvoClient {
	return &EvoClient{
		httpClient: httpClient,
		config:     config,
		ioStreams:  ioStreams,
	}
}

// DoAPI sends an API request and returns the raw response.
// It builds the full URL from config base URL + path, adds query params,
// marshals body to JSON if not nil, and returns the parsed response body
// along with the HTTP status code.
func (c *EvoClient) DoAPI(method, path string, params map[string]string, body interface{}) (*Response, error) {
	return c.DoAPIWithContext(context.Background(), method, path, params, body)
}

// DoAPIWithContext is like DoAPI but accepts a context for cancellation and
// value propagation (e.g., idempotency key).
func (c *EvoClient) DoAPIWithContext(ctx context.Context, method, path string, params map[string]string, body interface{}) (*Response, error) {
	// Build full URL from config base URL + path.
	baseURL := c.config.ResolveBaseURL("")
	fullURL, err := url.Parse(baseURL + path)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// Add query params.
	if len(params) > 0 {
		q := fullURL.Query()
		for k, v := range params {
			q.Set(k, v)
		}
		fullURL.RawQuery = q.Encode()
	}

	// Marshal body to JSON if not nil.
	var bodyReader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	// Create HTTP request with context.
	req, err := http.NewRequestWithContext(ctx, method, fullURL.String(), bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Send request.
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body.
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	// Parse response body as JSON map.
	var result map[string]interface{}
	if len(respBody) > 0 {
		if err := json.Unmarshal(respBody, &result); err != nil {
			// If body is not valid JSON, store raw text under "raw" key.
			result = map[string]interface{}{
				"raw": string(respBody),
			}
		}
	}

	return &Response{
		StatusCode: resp.StatusCode,
		Body:       result,
	}, nil
}

// CallAPI sends an API request and processes the response through the
// Evo Payment 5-step response processing flow:
//  1. Check HTTP status code — if not 200, return http_error
//  2. Response signature verification — already handled by SignatureTransport
//  3. Check action object — if present, set meta.actionRequired = true
//  4. Check result object — classify result.code using ClassifyResultCode
//  5. Extract business status — look for payment/capture/cancel/refund .status
func (c *EvoClient) CallAPI(method, path string, params map[string]string, body interface{}) (*output.Envelope, error) {
	return c.CallAPIWithContext(context.Background(), method, path, params, body)
}

// CallAPIWithContext is like CallAPI but accepts a context for cancellation and
// value propagation (e.g., idempotency key).
func (c *EvoClient) CallAPIWithContext(ctx context.Context, method, path string, params map[string]string, body interface{}) (*output.Envelope, error) {
	resp, err := c.DoAPIWithContext(ctx, method, path, params, body)
	if err != nil {
		return nil, err
	}

	return c.processResponse(resp)
}

// CallAPIWithBaseURL sends an API request using an explicit base URL instead of
// the config's default. This is used for LinkPay endpoints which have a different
// base URL than the main payment API.
func (c *EvoClient) CallAPIWithBaseURL(baseURL, method, path string, params map[string]string, body interface{}) (*output.Envelope, error) {
	resp, err := c.doAPIWithBaseURL(baseURL, context.Background(), method, path, params, body)
	if err != nil {
		return nil, err
	}

	return c.processResponse(resp)
}

// doAPIWithBaseURL is like DoAPIWithContext but uses an explicit base URL.
func (c *EvoClient) doAPIWithBaseURL(baseURL string, ctx context.Context, method, path string, params map[string]string, body interface{}) (*Response, error) {
	fullURL, err := url.Parse(baseURL + path)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// Add query params.
	if len(params) > 0 {
		q := fullURL.Query()
		for k, v := range params {
			q.Set(k, v)
		}
		fullURL.RawQuery = q.Encode()
	}

	// Marshal body to JSON if not nil.
	var bodyReader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	// Create HTTP request with context.
	req, err := http.NewRequestWithContext(ctx, method, fullURL.String(), bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Send request.
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body.
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	// Parse response body as JSON map.
	var result map[string]interface{}
	if len(respBody) > 0 {
		if err := json.Unmarshal(respBody, &result); err != nil {
			result = map[string]interface{}{
				"raw": string(respBody),
			}
		}
	}

	return &Response{
		StatusCode: resp.StatusCode,
		Body:       result,
	}, nil
}

// processResponse applies the Evo Payment 5-step response processing flow.
func (c *EvoClient) processResponse(resp *Response) (*output.Envelope, error) {

	// Step 1: Check HTTP status code.
	if resp.StatusCode != http.StatusOK {
		return nil, &HTTPError{
			StatusCode: resp.StatusCode,
			Body:       resp.Body,
		}
	}

	// Step 2: Signature verification is handled by SignatureTransport (no-op here).

	meta := &output.Meta{}

	// Step 3: Check action object.
	if _, hasAction := resp.Body["action"]; hasAction {
		meta.ActionRequired = true
	}

	// Step 4: Check result object and classify result.code.
	var resultExitCode int
	var resultErrType string
	if resultObj, ok := resp.Body["result"].(map[string]interface{}); ok {
		if code, ok := resultObj["code"].(string); ok {
			resultExitCode, resultErrType = output.ClassifyResultCode(code)
			if resultErrType != "" {
				// Non-success result code — return as structured error.
				msg, _ := resultObj["message"].(string)
				return nil, &ResultError{
					ExitCode: resultExitCode,
					ErrType:  resultErrType,
					Code:     code,
					Message:  msg,
				}
			}
		}
	}
	_ = resultExitCode // used above in error path

	// Step 5: Extract business status.
	meta.BusinessStatus = extractBusinessStatus(resp.Body)

	return &output.Envelope{
		OK:   true,
		Data: resp.Body,
		Meta: meta,
	}, nil
}

// extractBusinessStatus looks for payment.status, capture.status,
// cancel.status, or refund.status in the response body.
// Returns the first found status value.
func extractBusinessStatus(body map[string]interface{}) string {
	statusKeys := []string{"payment", "capture", "cancel", "refund"}
	for _, key := range statusKeys {
		if obj, ok := body[key].(map[string]interface{}); ok {
			if status, ok := obj["status"].(string); ok && status != "" {
				return status
			}
		}
	}
	return ""
}

// HTTPError represents a non-200 HTTP status code error.
type HTTPError struct {
	StatusCode int
	Body       map[string]interface{}
}

func (e *HTTPError) Error() string {
	// Try to extract result.message from body for more context.
	if e.Body != nil {
		if result, ok := e.Body["result"].(map[string]interface{}); ok {
			code, _ := result["code"].(string)
			msg, _ := result["message"].(string)
			if code != "" || msg != "" {
				return fmt.Sprintf("HTTP %d: %s %s", e.StatusCode, code, msg)
			}
		}
	}
	return fmt.Sprintf("HTTP %d", e.StatusCode)
}

// Type returns the structured error type.
func (e *HTTPError) Type() string {
	return "http_error"
}

// Code returns the HTTP status code as a string.
func (e *HTTPError) Code() string {
	return strconv.Itoa(e.StatusCode)
}

// ResultError represents a non-success result.code from the Evo Payment API.
type ResultError struct {
	ExitCode int
	ErrType  string
	Code     string
	Message  string
}

func (e *ResultError) Error() string {
	return fmt.Sprintf("%s: %s — %s", e.ErrType, e.Code, e.Message)
}

// Type returns the structured error type.
func (e *ResultError) Type() string {
	return e.ErrType
}
