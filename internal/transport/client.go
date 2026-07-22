package transport

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// userAgents matches the pool used by the Python reference tool (fr3ki.py).
var userAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/91.0.4472.124 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 Safari/537.36",
	"Mozilla/5.0 (X11; Ubuntu; Linux x86_64) Gecko/20100101 Firefox/89.0",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 14_6 like Mac OS X) AppleWebKit/605.1.15 Mobile/15E148",
	"Mozilla/5.0 (Linux; Android 11; Pixel 4) AppleWebKit/537.36 Chrome/91.0.4472.124 Mobile Safari/537.36",
}

const defaultTimeout = 10 * time.Second

// Response holds the parts of an HTTP response the fuzzer package needs.
type Response struct {
	StatusCode int
	Body       []byte
	Headers    http.Header
	// ProxyUsed is the proxy URL selected for this request, or empty if direct.
	ProxyUsed string
}

// Options configures a Client.
type Options struct {
	// Proxies is the pool to rotate through; nil or empty means direct.
	Proxies []string
	// Timeout is the per-request deadline. Zero defaults to 10 s.
	Timeout time.Duration
}

// Client is the single seam the fuzzer package calls through.
type Client struct {
	proxies   []string
	timeout   time.Duration
	transport *http.Transport
}

// New returns a ready-to-use Client.
func New(opts Options) *Client {
	t := opts.Timeout
	if t == 0 {
		t = defaultTimeout
	}
	return &Client{
		proxies:   opts.Proxies,
		timeout:   t,
		transport: &http.Transport{},
	}
}

// LoadProxies reads a newline-delimited file of proxy URLs, skipping blank lines.
func LoadProxies(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("transport: open proxy file: %w", err)
	}
	defer f.Close()

	var proxies []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if line := strings.TrimSpace(sc.Text()); line != "" {
			proxies = append(proxies, line)
		}
	}
	return proxies, sc.Err()
}

// Do executes an HTTP request. A random proxy and User-Agent are selected on
// every call. Headers in h are applied after the auto-selected User-Agent, so
// callers can override it.
func (c *Client) Do(ctx context.Context, method, rawURL string, body []byte, h map[string]string) (*Response, error) {
	var proxyUsed string
	tr := c.transport.Clone()

	if len(c.proxies) > 0 {
		proxyUsed = c.proxies[rand.Intn(len(c.proxies))]
		proxyURL, err := url.Parse(proxyUsed)
		if err != nil {
			return nil, fmt.Errorf("transport: parse proxy URL %q: %w", proxyUsed, err)
		}
		tr.Proxy = http.ProxyURL(proxyURL)
	}

	hc := &http.Client{
		Transport: tr,
		Timeout:   c.timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("transport: stopped after 10 redirects")
			}
			return nil
		},
	}

	var bodyReader io.Reader
	if len(body) > 0 {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, rawURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("transport: build request: %w", err)
	}

	req.Header.Set("User-Agent", userAgents[rand.Intn(len(userAgents))])
	for k, v := range h {
		req.Header.Set(k, v)
	}

	resp, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("transport: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("transport: read body: %w", err)
	}

	return &Response{
		StatusCode: resp.StatusCode,
		Body:       respBody,
		Headers:    resp.Header,
		ProxyUsed:  proxyUsed,
	}, nil
}
