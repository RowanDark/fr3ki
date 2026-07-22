package transport_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/RowanDark/fr3ki/internal/transport"
)

// knownUserAgents mirrors the pool declared in client.go.
var knownUserAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/91.0.4472.124 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 Safari/537.36",
	"Mozilla/5.0 (X11; Ubuntu; Linux x86_64) Gecko/20100101 Firefox/89.0",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 14_6 like Mac OS X) AppleWebKit/605.1.15 Mobile/15E148",
	"Mozilla/5.0 (Linux; Android 11; Pixel 4) AppleWebKit/537.36 Chrome/91.0.4472.124 Mobile Safari/537.36",
}

func inPool(ua string) bool {
	for _, a := range knownUserAgents {
		if a == ua {
			return true
		}
	}
	return false
}

// TestDo_SupportedMethods verifies that each HTTP method is forwarded correctly.
func TestDo_SupportedMethods(t *testing.T) {
	methods := []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
		http.MethodPatch,
		http.MethodHead,
		http.MethodOptions,
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			var gotMethod string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod = r.Method
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			c := transport.New(transport.Options{Timeout: 5 * time.Second})
			resp, err := c.Do(context.Background(), method, srv.URL, nil, nil)
			if err != nil {
				t.Fatalf("Do(%s) error: %v", method, err)
			}
			if resp.StatusCode != http.StatusOK {
				t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
			}
			if gotMethod != method {
				t.Errorf("server saw method %q, want %q", gotMethod, method)
			}
		})
	}
}

// TestDo_CustomHeadersArriveOnServer verifies that caller-supplied headers
// reach the target server unchanged.
func TestDo_CustomHeadersArriveOnServer(t *testing.T) {
	var mu sync.Mutex
	var gotHeaders http.Header

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gotHeaders = r.Header.Clone()
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := transport.New(transport.Options{Timeout: 5 * time.Second})
	customHeaders := map[string]string{
		"X-Custom-One": "alpha",
		"X-Custom-Two": "beta",
	}
	_, err := c.Do(context.Background(), http.MethodGet, srv.URL, nil, customHeaders)
	if err != nil {
		t.Fatalf("Do error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	for k, v := range customHeaders {
		if got := gotHeaders.Get(k); got != v {
			t.Errorf("header %q = %q, want %q", k, got, v)
		}
	}
}

// TestDo_UserAgentFromPool verifies that the auto-selected User-Agent is
// always one of the five strings in the known pool.
func TestDo_UserAgentFromPool(t *testing.T) {
	var mu sync.Mutex
	var gotUA string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gotUA = r.Header.Get("User-Agent")
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := transport.New(transport.Options{Timeout: 5 * time.Second})
	for i := 0; i < 20; i++ {
		_, err := c.Do(context.Background(), http.MethodGet, srv.URL, nil, nil)
		if err != nil {
			t.Fatalf("Do error: %v", err)
		}
		mu.Lock()
		ua := gotUA
		mu.Unlock()
		if !inPool(ua) {
			t.Fatalf("User-Agent %q not in known pool", ua)
		}
	}
}

// TestDo_CustomHeaderOverridesUserAgent verifies that a caller-supplied
// User-Agent header wins over the auto-selected one.
func TestDo_CustomHeaderOverridesUserAgent(t *testing.T) {
	var mu sync.Mutex
	var gotUA string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gotUA = r.Header.Get("User-Agent")
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := transport.New(transport.Options{Timeout: 5 * time.Second})
	customUA := "MyCustomAgent/1.0"
	_, err := c.Do(context.Background(), http.MethodGet, srv.URL, nil, map[string]string{
		"User-Agent": customUA,
	})
	if err != nil {
		t.Fatalf("Do error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if gotUA != customUA {
		t.Errorf("User-Agent = %q, want %q", gotUA, customUA)
	}
}

// TestDo_ProxySelectionCoversAllProxies verifies that random proxy rotation
// distributes requests across every proxy in the list over enough trials.
//
// Each httptest server acts as a minimal HTTP proxy: for plain-HTTP requests
// the Go client sends the full URL in the Request-URI, the "proxy" server
// records the hit and returns 200, which the client treats as success.
func TestDo_ProxySelectionCoversAllProxies(t *testing.T) {
	const proxyCount = 3
	const trials = 120

	contacted := make([]int32, proxyCount)
	var mu sync.Mutex

	var proxySrvs []*httptest.Server
	for i := 0; i < proxyCount; i++ {
		idx := i
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			mu.Lock()
			contacted[idx]++
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
		}))
		proxySrvs = append(proxySrvs, srv)
	}
	defer func() {
		for _, s := range proxySrvs {
			s.Close()
		}
	}()

	proxies := make([]string, proxyCount)
	for i, s := range proxySrvs {
		proxies[i] = s.URL
	}

	c := transport.New(transport.Options{
		Proxies: proxies,
		Timeout: 5 * time.Second,
	})

	for i := 0; i < trials; i++ {
		// Target need not be reachable; the proxy server responds directly.
		c.Do(context.Background(), http.MethodGet, "http://example.com/fuzz", nil, nil) //nolint:errcheck
	}

	mu.Lock()
	defer mu.Unlock()
	for i, count := range contacted {
		if count == 0 {
			t.Errorf("proxy[%d] (%s) was never selected in %d requests", i, proxies[i], trials)
		}
	}
}

// TestDo_ProxyUsedReturned verifies that the Response.ProxyUsed field is
// populated with the proxy URL that was chosen for the request.
func TestDo_ProxyUsedReturned(t *testing.T) {
	proxySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer proxySrv.Close()

	c := transport.New(transport.Options{
		Proxies: []string{proxySrv.URL},
		Timeout: 5 * time.Second,
	})

	resp, err := c.Do(context.Background(), http.MethodGet, "http://example.com/test", nil, nil)
	if err != nil {
		t.Fatalf("Do error: %v", err)
	}
	if resp.ProxyUsed != proxySrv.URL {
		t.Errorf("ProxyUsed = %q, want %q", resp.ProxyUsed, proxySrv.URL)
	}
}

// TestDo_NoProxyUsedWhenEmpty verifies that ProxyUsed is empty when no proxy
// list is configured.
func TestDo_NoProxyUsedWhenEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := transport.New(transport.Options{Timeout: 5 * time.Second})
	resp, err := c.Do(context.Background(), http.MethodGet, srv.URL, nil, nil)
	if err != nil {
		t.Fatalf("Do error: %v", err)
	}
	if resp.ProxyUsed != "" {
		t.Errorf("ProxyUsed = %q, want empty string", resp.ProxyUsed)
	}
}

// TestDo_FollowsRedirects verifies that the client follows HTTP redirects.
func TestDo_FollowsRedirects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			http.Redirect(w, r, "/end", http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("arrived"))
	}))
	defer srv.Close()

	c := transport.New(transport.Options{Timeout: 5 * time.Second})
	resp, err := c.Do(context.Background(), http.MethodGet, srv.URL+"/start", nil, nil)
	if err != nil {
		t.Fatalf("Do error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d (redirect not followed)", resp.StatusCode, http.StatusOK)
	}
	if string(resp.Body) != "arrived" {
		t.Errorf("body = %q, want %q", resp.Body, "arrived")
	}
}

// TestDo_SendsRequestBody verifies that a non-nil body is forwarded to the server.
func TestDo_SendsRequestBody(t *testing.T) {
	var mu sync.Mutex
	var gotBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		gotBody = b
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := transport.New(transport.Options{Timeout: 5 * time.Second})
	payload := []byte(`{"key":"value"}`)
	_, err := c.Do(context.Background(), http.MethodPost, srv.URL, payload, map[string]string{
		"Content-Type": "application/json",
	})
	if err != nil {
		t.Fatalf("Do error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if string(gotBody) != string(payload) {
		t.Errorf("body = %q, want %q", gotBody, payload)
	}
}

// TestLoadProxies_ValidFile verifies that a well-formed proxy file is parsed
// correctly, with blank lines and whitespace-only lines skipped.
func TestLoadProxies_ValidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "proxies.txt")
	content := "http://proxy1.example.com:8080\n\nhttp://proxy2.example.com:8080\n  \nhttp://proxy3.example.com:8080\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	proxies, err := transport.LoadProxies(path)
	if err != nil {
		t.Fatalf("LoadProxies error: %v", err)
	}
	want := []string{
		"http://proxy1.example.com:8080",
		"http://proxy2.example.com:8080",
		"http://proxy3.example.com:8080",
	}
	if len(proxies) != len(want) {
		t.Fatalf("got %d proxies, want %d", len(proxies), len(want))
	}
	for i, p := range proxies {
		if p != want[i] {
			t.Errorf("proxy[%d] = %q, want %q", i, p, want[i])
		}
	}
}

// TestLoadProxies_MissingFile verifies that a missing file returns a wrapped error.
func TestLoadProxies_MissingFile(t *testing.T) {
	_, err := transport.LoadProxies("/nonexistent/proxies.txt")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
	if !strings.Contains(err.Error(), "transport:") {
		t.Errorf("error %q does not include 'transport:' prefix", err.Error())
	}
}

// TestLoadProxies_EmptyFile verifies that an empty file returns an empty slice.
func TestLoadProxies_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.txt")
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	proxies, err := transport.LoadProxies(path)
	if err != nil {
		t.Fatalf("LoadProxies error: %v", err)
	}
	if len(proxies) != 0 {
		t.Errorf("got %d proxies, want 0", len(proxies))
	}
}
