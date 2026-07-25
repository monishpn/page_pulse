package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/monishpn/page_pulse/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/container"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/http/response"
)

type fakeAuditService struct {
	auditURLFunc func(ctx *gofr.Context, url string) (*models.AuditResponse, *models.CustomError)
	called       bool
	calledWith   string
}

func (f *fakeAuditService) AuditURL(ctx *gofr.Context, url string) (*models.AuditResponse, *models.CustomError) {
	f.called = true
	f.calledWith = url

	return f.auditURLFunc(ctx, url)
}

func newTestContext(t *testing.T, body string) *gofr.Context {
	t.Helper()

	mockContainer, _ := container.NewMockContainer(t)

	req := httptest.NewRequest(http.MethodPost, "/audit", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	return &gofr.Context{
		Context:   context.Background(),
		Request:   gofrHTTP.NewRequest(req),
		Container: mockContainer,
	}
}

func TestHandler_AuditURL(t *testing.T) {
	sampleAudit := &models.AuditResponse{URL: "https://example.com", StatusCode: 200}

	tests := []struct {
		name              string
		body              string
		auditURLFunc      func(ctx *gofr.Context, url string) (*models.AuditResponse, *models.CustomError)
		wantServiceCalled bool
		wantErr           bool
		wantResp          any
	}{
		{
			name:              "malformed JSON body is rejected before reaching the service",
			body:              `{"url":`,
			wantServiceCalled: false,
			wantErr:           true,
		},
		{
			name:              "invalid url is rejected before reaching the service",
			body:              `{"url":"not-a-url"}`,
			wantServiceCalled: false,
			wantErr:           true,
		},
		{
			name:              "localhost url is rejected before reaching the service",
			body:              `{"url":"http://localhost"}`,
			wantServiceCalled: false,
			wantErr:           true,
		},
		{
			name: "service error is propagated",
			body: `{"url":"https://example.com"}`,
			auditURLFunc: func(_ *gofr.Context, _ string) (*models.AuditResponse, *models.CustomError) {
				return nil, &models.CustomError{Message: "boom", Code: http.StatusBadGateway}
			},
			wantServiceCalled: true,
			wantErr:           true,
		},
		{
			name: "successful audit is wrapped in response.Raw",
			body: `{"url":"https://example.com"}`,
			auditURLFunc: func(_ *gofr.Context, _ string) (*models.AuditResponse, *models.CustomError) {
				return sampleAudit, nil
			},
			wantServiceCalled: true,
			wantErr:           false,
			wantResp:          response.Raw{Data: sampleAudit},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeAuditService{auditURLFunc: tt.auditURLFunc}
			h := New(fake)

			ctx := newTestContext(t, tt.body)

			got, err := h.AuditURL(ctx)

			assert.Equal(t, tt.wantServiceCalled, fake.called)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantResp, got)
		})
	}
}
