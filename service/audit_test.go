package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/monishpn/page_pulse/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/container"
	gofrHTTP "gofr.dev/pkg/gofr/http"
)

const sampleHTML = `<!DOCTYPE html>
<html>
<head>
<title>Test Page</title>
<meta name="description" content="A page for testing">
</head>
<body>
<h1>Hello World</h1>
</body>
</html>`

type fakeCacheStore struct {
	checkAuditFunc func(ctx *gofr.Context, key string) (bool, error)
	getAuditFunc   func(ctx *gofr.Context, key string) (*models.AuditResponse, error)
	putAuditFunc   func(ctx *gofr.Context, a *models.AuditResponse) error
	putCalled      bool
	putAuditArg    *models.AuditResponse
}

func (f *fakeCacheStore) CheckAudit(ctx *gofr.Context, key string) (bool, error) {
	if f.checkAuditFunc != nil {
		return f.checkAuditFunc(ctx, key)
	}

	return false, nil
}

func (f *fakeCacheStore) GetAudit(ctx *gofr.Context, key string) (*models.AuditResponse, error) {
	if f.getAuditFunc != nil {
		return f.getAuditFunc(ctx, key)
	}

	return nil, nil
}

func (f *fakeCacheStore) PutAudit(ctx *gofr.Context, a *models.AuditResponse) error {
	f.putCalled = true
	f.putAuditArg = a

	if f.putAuditFunc != nil {
		return f.putAuditFunc(ctx, a)
	}

	return nil
}

func newTestContext(t *testing.T) *gofr.Context {
	t.Helper()

	mockContainer, _ := container.NewMockContainer(t)

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)

	return &gofr.Context{
		Context:   context.Background(),
		Request:   gofrHTTP.NewRequest(req),
		Container: mockContainer,
	}
}

func TestService_AuditURL_CacheHit(t *testing.T) {
	cachedAudit := &models.AuditResponse{URL: "https://cached.example.com", PageTitle: "Cached Title"}

	fake := &fakeCacheStore{
		checkAuditFunc: func(_ *gofr.Context, _ string) (bool, error) { return true, nil },
		getAuditFunc:   func(_ *gofr.Context, _ string) (*models.AuditResponse, error) { return cachedAudit, nil },
	}

	svc := New("5", fake)

	// This host is never actually reachable; if the cache-hit short-circuit
	// didn't work, this test would fail (or hang) trying to fetch it.
	got, err := svc.AuditURL(newTestContext(t), "https://cached.example.com")

	require.Nil(t, err)
	assert.Equal(t, cachedAudit, got)
	assert.False(t, fake.putCalled, "a cache hit should not write back to the cache")
}

func TestService_AuditURL_FetchAndCache(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Server", "test-server")
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sampleHTML))
	}))
	defer srv.Close()

	fake := &fakeCacheStore{}
	svc := New("5", fake)

	got, err := svc.AuditURL(newTestContext(t), srv.URL)

	require.Nil(t, err)
	require.NotNil(t, got)
	assert.Equal(t, http.StatusOK, got.StatusCode)
	assert.Equal(t, "Test Page", got.PageTitle)
	assert.Equal(t, "A page for testing", got.MetaDescription)
	assert.Equal(t, "Hello World", got.H1)
	assert.Equal(t, "test-server", got.Server)
	assert.Equal(t, "text/html", got.ContentType)
	assert.False(t, got.HTTPS)
	assert.Equal(t, srv.URL, got.FinalURL)

	assert.True(t, fake.putCalled, "a cache miss followed by a successful fetch should be cached")
	assert.Equal(t, got, fake.putAuditArg)
}

func TestService_AuditURL_CacheWriteFailureIsNotFatal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sampleHTML))
	}))
	defer srv.Close()

	fake := &fakeCacheStore{
		putAuditFunc: func(_ *gofr.Context, _ *models.AuditResponse) error {
			return errors.New("redis is down")
		},
	}

	svc := New("5", fake)

	got, err := svc.AuditURL(newTestContext(t), srv.URL)

	require.Nil(t, err)
	assert.NotNil(t, got)
}

func TestService_AuditURL_Errors(t *testing.T) {
	tests := []struct {
		name     string
		target   func() string
		svc      func(fake *fakeCacheStore) *service
		wantCode int
	}{
		{
			name:     "malformed url fails before any request is made",
			target:   func() string { return "http://exa mple.com" },
			svc:      func(fake *fakeCacheStore) *service { return New("5", fake) },
			wantCode: http.StatusBadRequest,
		},
		{
			name: "unreachable host returns an upstream error",
			target: func() string {
				srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
				srv.Close() // connection refused: the listener is gone before the request is made

				return srv.URL
			},
			svc:      func(fake *fakeCacheStore) *service { return New("5", fake) },
			wantCode: http.StatusBadGateway,
		},
		{
			name: "slow upstream times out",
			target: func() string {
				srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
					time.Sleep(100 * time.Millisecond)
				}))
				t.Cleanup(srv.Close)

				return srv.URL
			},
			svc: func(fake *fakeCacheStore) *service {
				return &service{requestTimeout: 10 * time.Millisecond, cacheStore: fake}
			},
			wantCode: http.StatusGatewayTimeout,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeCacheStore{}
			svc := tt.svc(fake)

			got, err := svc.AuditURL(newTestContext(t), tt.target())

			require.NotNil(t, err)
			require.Nil(t, got)
			assert.Equal(t, tt.wantCode, err.Code)
		})
	}
}
