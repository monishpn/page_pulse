package cache

import (
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/monishpn/page_pulse/models"
	"github.com/redis/go-redis/v9"
	"gofr.dev/pkg/gofr"
)

const keyPrefix = "audit:"

type store struct {
	requestTTL time.Duration
}

func New(requestTTL string) *store {
	requestTTLInt, err := strconv.Atoi(requestTTL)
	if err != nil {
		// If TTL not in int, set it back to default
		requestTTLInt = 10
	}

	return &store{
		requestTTL: time.Duration(requestTTLInt) * time.Minute,
	}
}

func (s *store) GetAudit(ctx *gofr.Context, key string) (*models.AuditResponse, error) {
	raw, err := ctx.Redis.Get(ctx, keyPrefix+key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}

		return nil, err
	}

	var audit models.AuditResponse
	if err := json.Unmarshal([]byte(raw), &audit); err != nil {
		return nil, err
	}

	return &audit, nil
}

func (s *store) PutAudit(ctx *gofr.Context, a *models.AuditResponse) error {
	data, err := json.Marshal(a)
	if err != nil {
		return err
	}

	return ctx.Redis.Set(ctx, keyPrefix+a.URL, data, s.requestTTL).Err()
}

func (s *store) CheckAudit(ctx *gofr.Context, key string) (bool, error) {
	n, err := ctx.Redis.Exists(ctx, keyPrefix+key).Result()
	if err != nil {
		return false, err
	}

	return n > 0, nil
}
