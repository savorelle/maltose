package mclient

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/graingo/maltose/internal/intlog"
)

// Request is the struct for client request.
type Request struct {
	*http.Request                                   // Request is the underlying http.Request object.
	client         *Client                          // The client that creates this request.
	response       *Response                        // The response object of this request.
	ctx            context.Context                  // Context for the request.
	timeout        time.Duration                    // Timeout for the request.
	retryCount     int                              // Retry count for the request.
	retryInterval  time.Duration                    // Retry interval for the request.
	header         map[string]string                // Custom header map.
	query          map[string]string                // Custom query map.
	form           map[string]string                // Custom form map.
	body           []byte                           // Custom body content.
	contentType    string                           // Content type of the request.
	middlewares    []MiddlewareFunc                 // Middleware functions.
	queryParams    url.Values                       // Query parameters.
	formParams     url.Values                       // Form parameters.
	retryCondition func(*http.Response, error) bool // Retry condition.
}

// GetResponse returns the response object of this request.
func (r *Request) GetResponse() *Response {
	return r.response
}

// SetResponse sets the response object for this request.
func (r *Request) SetResponse(resp *Response) {
	r.response = resp
}

// -----------------------------------------------------------------------------
// Client Methods
// -----------------------------------------------------------------------------

// NewRequest creates and returns a new request object.
func (c *Client) NewRequest() *Request {
	return &Request{
		client:      c,
		middlewares: make([]MiddlewareFunc, 0),
		queryParams: make(url.Values),
		formParams:  make(url.Values),
	}
}

// R returns a new request object bound to this client for chain calls.
func (c *Client) R() *Request {
	return c.NewRequest()
}

// -----------------------------------------------------------------------------
// Request Basic Setup Methods
// -----------------------------------------------------------------------------

// SetContext sets the context for the request.
func (r *Request) SetContext(ctx context.Context) *Request {
	if r.Request == nil {
		r.Request = &http.Request{}
	}

	if ctx != nil {
		r.Request = r.Request.WithContext(ctx)
	}

	return r
}

// Method sets the HTTP method for the request.
func (r *Request) Method(method string) *Request {
	if r.Request == nil {
		r.Request = &http.Request{
			Header: make(http.Header),
		}
	}
	r.Request.Method = method
	return r
}

// URL sets the request URL.
func (r *Request) URL(url string) *Request {
	if r.Request == nil {
		r.Request = &http.Request{
			Header: make(http.Header),
		}
	}
	parsed, err := r.Request.URL.Parse(url)
	if err == nil {
		r.Request.URL = parsed
	}
	return r
}

// -----------------------------------------------------------------------------
// Header Related Methods
// -----------------------------------------------------------------------------

// Header sets an HTTP header for the request.
func (r *Request) Header(key, value string) *Request {
	if r.Request == nil {
		r.Request = &http.Request{
			Header: make(http.Header),
		}
	}
	r.Request.Header.Set(key, value)
	return r
}

// SetHeader sets a header key-value pair for the request.
// This is an alias of Header method for better chain API compatibility.
func (r *Request) SetHeader(key, value string) *Request {
	return r.Header(key, value)
}

// SetHeaders sets multiple headers at once.
func (r *Request) SetHeaders(headers map[string]string) *Request {
	for k, v := range headers {
		r.Header(k, v)
	}
	return r
}

// ContentType sets the Content-Type header for the request.
func (r *Request) ContentType(contentType string) *Request {
	return r.Header("Content-Type", contentType)
}

// -----------------------------------------------------------------------------
// Request Body Methods
// -----------------------------------------------------------------------------

// SetBody sets the request body.
func (r *Request) SetBody(body any) *Request {
	return r.Data(body)
}

// Data sets the request data.
func (r *Request) Data(data any) *Request {
	if r.Request == nil {
		r.Request = &http.Request{
			Header: make(http.Header),
		}
	}

	switch d := data.(type) {
	case string:
		r.Request.Body = io.NopCloser(strings.NewReader(d))
	case []byte:
		r.Request.Body = io.NopCloser(bytes.NewReader(d))
	case io.Reader:
		r.Request.Body = io.NopCloser(d)
	default:
		// Try JSON encoding for other types
		jsonBytes, err := json.Marshal(data)
		if err != nil {
			// Log error but continue execution
			// Using request context if available, otherwise fallback to background context
			ctx := context.Background()
			if r.Request != nil && r.Request.Context() != nil {
				ctx = r.Request.Context()
			}
			intlog.Error(ctx, "JSON marshal failed:", err)
			return r
		}
		r.Request.Body = io.NopCloser(bytes.NewReader(jsonBytes))
		if r.Request.Header.Get("Content-Type") == "" {
			r.ContentType("application/json")
		}
	}
	return r
}

// -----------------------------------------------------------------------------
// Query Parameter Methods
// -----------------------------------------------------------------------------

// SetQuery sets a query parameter for the request.
func (r *Request) SetQuery(key, value string) *Request {
	r.queryParams.Set(key, value)
	return r
}

// SetQueryMap sets multiple query parameters from a map.
func (r *Request) SetQueryMap(params map[string]string) *Request {
	for k, v := range params {
		r.queryParams.Set(k, v)
	}
	return r
}

// -----------------------------------------------------------------------------
// Form Parameter Methods
// -----------------------------------------------------------------------------

// SetForm sets a form parameter for the request.
func (r *Request) SetForm(key, value string) *Request {
	r.formParams.Set(key, value)
	return r
}

// SetFormMap sets multiple form parameters from a map.
func (r *Request) SetFormMap(params map[string]string) *Request {
	for k, v := range params {
		r.formParams.Set(k, v)
	}
	return r
}

// -----------------------------------------------------------------------------
// Response Processing Methods
// -----------------------------------------------------------------------------

// SetResult sets the result object for successful response.
func (r *Request) SetResult(result any) *Request {
	if r.response != nil {
		r.response.SetResult(result)
	}
	return r
}

// SetError sets the error result object for error response.
func (r *Request) SetError(err any) *Request {
	if r.response != nil {
		r.response.SetError(err)
	}
	return r
}

// -----------------------------------------------------------------------------
// Retry Configuration Methods
// -----------------------------------------------------------------------------

// SetRetry sets retry count and interval.
func (r *Request) SetRetry(count int, interval time.Duration) *Request {
	r.retryCount = count
	r.retryInterval = interval
	return r
}

// SetRetryCondition sets a custom retry condition function.
// The function takes the HTTP response and error as input and returns
// true if the request should be retried.
func (r *Request) SetRetryCondition(condition func(*http.Response, error) bool) *Request {
	r.retryCondition = condition
	return r
}

// shouldRetry determines if a request should be retried based on the response and error.
func (r *Request) shouldRetry(resp *http.Response, err error) bool {
	// Use custom condition if provided
	if r.retryCondition != nil {
		return r.retryCondition(resp, err)
	}

	// Default retry condition
	if err != nil {
		// Retry on network/connection errors
		return true
	}

	if resp != nil {
		// Retry on 5xx (server errors) and 429 (too many requests)
		return resp.StatusCode >= 500 || resp.StatusCode == 429
	}

	return false
}

// -----------------------------------------------------------------------------
// HTTP Request Methods
// -----------------------------------------------------------------------------

// GET sets the method to GET and executes the request.
func (r *Request) GET(url string) (*Response, error) {
	return r.Method(http.MethodGet).Send(url)
}

// POST sets the method to POST and executes the request.
func (r *Request) POST(url string) (*Response, error) {
	return r.Method(http.MethodPost).Send(url)
}

// PUT sets the method to PUT and executes the request.
func (r *Request) PUT(url string) (*Response, error) {
	return r.Method(http.MethodPut).Send(url)
}

// DELETE sets the method to DELETE and executes the request.
func (r *Request) DELETE(url string) (*Response, error) {
	return r.Method(http.MethodDelete).Send(url)
}

// PATCH sets the method to PATCH and executes the request.
func (r *Request) PATCH(url string) (*Response, error) {
	return r.Method(http.MethodPatch).Send(url)
}

// HEAD sets the method to HEAD and executes the request.
func (r *Request) HEAD(url string) (*Response, error) {
	return r.Method(http.MethodHead).Send(url)
}

// OPTIONS sets the method to OPTIONS and executes the request.
func (r *Request) OPTIONS(url string) (*Response, error) {
	return r.Method(http.MethodOptions).Send(url)
}

// Send performs a request with the chain style API.
// If the method is not specified, it defaults to GET.
func (r *Request) Send(url string) (*Response, error) {
	if r.Request == nil || r.Request.Method == "" {
		// Default to GET method if not specified
		return r.DoRequest(r.Request.Context(), http.MethodGet, url)
	}

	return r.DoRequest(r.Request.Context(), r.Request.Method, url)
}

// DoRequest sends the request and returns the response.
func (r *Request) DoRequest(ctx context.Context, method string, urlPath string) (*Response, error) {
	var (
		err      error
		resp     *Response
		attempts = 0
	)

	// Start with at least one attempt (0 retries)
	maxAttempts := r.retryCount + 1
	if maxAttempts <= 0 {
		maxAttempts = 1
	}

	for attempts < maxAttempts {
		attempts++

		// Create a new request for each attempt
		resp, err = r.attemptRequest(ctx, method, urlPath)

		// Break if we shouldn't retry
		if !r.shouldRetry(resp.Response, err) || attempts >= maxAttempts {
			break
		}

		// Close the response before retry if it exists
		if resp != nil {
			resp.Close()
			resp = nil
		}

		// Log retry attempt
		if r.Request != nil && r.Request.Context() != nil {
			intlog.Printf(r.Request.Context(), "Retrying request (attempt %d/%d) after error: %v",
				attempts, maxAttempts, err)
		}

		// Wait before retry if interval is set
		if r.retryInterval > 0 {
			select {
			case <-time.After(r.retryInterval):
				// Continue after waiting
			case <-ctx.Done():
				// Context cancelled during wait
				return nil, ctx.Err()
			}
		}
	}

	if err != nil {
		return nil, err
	}

	// Parse response if needed
	if err := resp.ParseResponse(); err != nil {
		resp.Close()
		return nil, err
	}

	return resp, nil
}

// attemptRequest makes a single attempt to execute the request
func (r *Request) attemptRequest(ctx context.Context, method string, urlPath string) (*Response, error) {
	var (
		req *http.Request
		err error
	)

	// Prepare the request URL
	fullURL := urlPath
	if r.client.config.BaseURL != "" && !strings.HasPrefix(urlPath, "http://") && !strings.HasPrefix(urlPath, "https://") {
		baseURL := r.client.config.BaseURL

		// Ensure there's a single slash between baseURL and urlPath
		if !strings.HasSuffix(baseURL, "/") && !strings.HasPrefix(urlPath, "/") {
			baseURL = baseURL + "/"
		} else if strings.HasSuffix(baseURL, "/") && strings.HasPrefix(urlPath, "/") {
			urlPath = urlPath[1:]
		}

		fullURL = baseURL + urlPath
	}

	// Process query parameters
	if len(r.queryParams) > 0 {
		if strings.Contains(fullURL, "?") {
			fullURL = fullURL + "&" + r.queryParams.Encode()
		} else {
			fullURL = fullURL + "?" + r.queryParams.Encode()
		}
	}

	// Process form parameters
	var body io.Reader
	if len(r.formParams) > 0 {
		// Prioritize form data
		body = strings.NewReader(r.formParams.Encode())
		if r.Request == nil {
			r.Request = &http.Request{
				Header: make(http.Header),
			}
		}
		r.ContentType("application/x-www-form-urlencoded")
	} else if r.Request != nil && r.Request.Body != nil {
		// For retries, we need to make body re-readable
		if bodyBytes, err := io.ReadAll(r.Request.Body); err == nil {
			r.Request.Body.Close()
			body = bytes.NewReader(bodyBytes)
			r.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		} else {
			// If we can't read the body, use it directly
			// Note: this might cause issues with retries
			body = r.Request.Body
		}
	}

	// Create the HTTP request
	req, err = http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return nil, err
	}

	// Set headers from the client config
	if r.client.config.Header != nil {
		for k, v := range r.client.config.Header {
			if len(v) > 0 {
				req.Header.Set(k, v[0])
			}
		}
	}

	// Set headers from the request
	if r.Request != nil && r.Request.Header != nil {
		for k, v := range r.Request.Header {
			if len(v) > 0 {
				req.Header.Set(k, v[0])
			}
		}
	}

	// Set the updated http.Request in our Request object
	r.Request = req

	// Create a Response placeholder that will be filled by middleware
	var response *Response

	// Prepare the middleware chain
	middlewares := append(r.client.middlewares, r.middlewares...)
	if len(middlewares) > 0 {
		// Base handler - direct HTTP client call without middleware
		handler := func(req *Request) (*Response, error) {
			// At this point, use the underlying http.Request
			httpResp, err := r.client.Do(req.Request)
			if err != nil {
				return nil, err
			}

			// Create Response object
			return &Response{
				Response: httpResp,
			}, nil
		}

		// Apply middlewares in reverse order
		for i := len(middlewares) - 1; i >= 0; i-- {
			handler = middlewares[i](handler)
		}

		// Execute the middleware chain with our Request object
		response, err = handler(r)
	} else {
		// Direct request without middleware
		httpResp, err := r.client.Do(req)
		if err != nil {
			return nil, err
		}

		// Create Response object
		response = &Response{
			Response: httpResp,
		}
	}

	// Handle errors
	if err != nil {
		return nil, err
	}

	// Set response to request
	r.SetResponse(response)

	return response, nil
}
