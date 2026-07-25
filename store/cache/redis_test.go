package cache

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/monishpn/page_pulse/models"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/container"
	gofrHTTP "gofr.dev/pkg/gofr/http"
)

func newTestContext(t *testing.T) (*gofr.Context, *container.Mocks) {
	t.Helper()

	mockContainer, mocks := container.NewMockContainer(t)

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)

	ctx := &gofr.Context{
		Context:   context.Background(),
		Request:   gofrHTTP.NewRequest(req),
		Container: mockContainer,
	}

	return ctx, mocks
}

func TestStore_GetAudit(t *testing.T) {
	sampleAudit := &models.AuditResponse{
		URL:        "https://example.com",
		StatusCode: 200,
		PageTitle:  "Example",
		AuditedAt:  "2026-07-25T00:00:00Z",
	}

	sampleJSON, err := json.Marshal(sampleAudit)
	require.NoError(t, err)

	tests := []struct {
		name       string
		key        string
		mockExpect func(mocks *container.Mocks)
		wantAudit  *models.AuditResponse
		wantErr    bool
	}{
		{
			name: "cache hit returns the stored audit",
			key:  "https://example.com",
			mockExpect: func(mocks *container.Mocks) {
				mocks.Redis.EXPECT().
					Get(gomock.Any(), "audit:https://example.com").
					Return(redis.NewStringResult(string(sampleJSON), nil))
			},
			wantAudit: sampleAudit,
			wantErr:   false,
		},
		{
			name: "cache miss returns nil without an error",
			key:  "https://missing.com",
			mockExpect: func(mocks *container.Mocks) {
				mocks.Redis.EXPECT().
					Get(gomock.Any(), "audit:https://missing.com").
					Return(redis.NewStringResult("", redis.Nil))
			},
			wantAudit: nil,
			wantErr:   false,
		},
		{
			name: "redis error propagates",
			key:  "https://example.com",
			mockExpect: func(mocks *container.Mocks) {
				mocks.Redis.EXPECT().
					Get(gomock.Any(), "audit:https://example.com").
					Return(redis.NewStringResult("", errors.New("connection refused")))
			},
			wantAudit: nil,
			wantErr:   true,
		},
		{
			name: "malformed cached value returns an unmarshal error",
			key:  "https://example.com",
			mockExpect: func(mocks *container.Mocks) {
				mocks.Redis.EXPECT().
					Get(gomock.Any(), "audit:https://example.com").
					Return(redis.NewStringResult("not-json", nil))
			},
			wantAudit: nil,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, mocks := newTestContext(t)
			tt.mockExpect(mocks)

			s := New("10")

			got, err := s.GetAudit(ctx, tt.key)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tt.wantAudit, got)
		})
	}
}

func TestStore_PutAudit(t *testing.T) {
	audit := &models.AuditResponse{
		URL:        "https://example.com",
		StatusCode: 200,
	}

	tests := []struct {
		name       string
		mockExpect func(mocks *container.Mocks)
		wantErr    bool
	}{
		{
			name: "successful write uses the configured TTL",
			mockExpect: func(mocks *container.Mocks) {
				mocks.Redis.EXPECT().
					Set(gomock.Any(), "audit:https://example.com", gomock.Any(), 10*time.Minute).
					Return(redis.NewStatusResult("OK", nil))
			},
			wantErr: false,
		},
		{
			name: "redis error propagates",
			mockExpect: func(mocks *container.Mocks) {
				mocks.Redis.EXPECT().
					Set(gomock.Any(), "audit:https://example.com", gomock.Any(), 10*time.Minute).
					Return(redis.NewStatusResult("", errors.New("connection refused")))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, mocks := newTestContext(t)
			tt.mockExpect(mocks)

			s := New("10")

			err := s.PutAudit(ctx, audit)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestStore_CheckAudit(t *testing.T) {
	tests := []struct {
		name       string
		mockExpect func(mocks *container.Mocks)
		want       bool
		wantErr    bool
	}{
		{
			name: "key exists",
			mockExpect: func(mocks *container.Mocks) {
				mocks.Redis.EXPECT().
					Exists(gomock.Any(), "audit:https://example.com").
					Return(redis.NewIntResult(1, nil))
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "key does not exist",
			mockExpect: func(mocks *container.Mocks) {
				mocks.Redis.EXPECT().
					Exists(gomock.Any(), "audit:https://example.com").
					Return(redis.NewIntResult(0, nil))
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "redis error propagates",
			mockExpect: func(mocks *container.Mocks) {
				mocks.Redis.EXPECT().
					Exists(gomock.Any(), "audit:https://example.com").
					Return(redis.NewIntResult(0, errors.New("connection refused")))
			},
			want:    false,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, mocks := newTestContext(t)
			tt.mockExpect(mocks)

			s := New("10")

			got, err := s.CheckAudit(ctx, "https://example.com")

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tt.want, got)
		})
	}
}
