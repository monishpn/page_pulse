package main

import (
	"gofr.dev/pkg/gofr"

	"github.com/monishpn/page_pulse/handler"
	"github.com/monishpn/page_pulse/middleware"
	"github.com/monishpn/page_pulse/service"
	"github.com/monishpn/page_pulse/store/cache"
)

func main() {
	app := gofr.New()

	requestTimeout := app.Config.GetOrDefault("URL_REQUEST_TIMEOUT", "30")
	requestTTL := app.Config.GetOrDefault("REQUEST_TTL", "10")
	rateLimitMax := app.Config.GetOrDefault("RATE_LIMIT_MAX", "10")
	rateLimitWindowSeconds := app.Config.GetOrDefault("RATE_LIMIT_WINDOW_SECONDS", "60")
	trustProxyHeaders := app.Config.GetOrDefault("TRUST_PROXY_HEADERS", "false") == "true"

	rateLimiter := middleware.NewRateLimit(rateLimitMax, rateLimitWindowSeconds, trustProxyHeaders, "/audit")

	// The DI-managed Redis connection (ctx.Redis) is only reachable once
	// the app has started, so OnStart injects it into the already-wired
	// rate limiter for it to read lazily on each request.
	app.OnStart(func(ctx *gofr.Context) error {
		rateLimiter.SetContext(ctx)
		return nil
	})

	app.UseMiddleware(rateLimiter.Middleware)

	cacheStore := cache.New(requestTTL)
	auditSvc := service.New(requestTimeout, cacheStore)
	auditHdlr := handler.New(auditSvc)

	// register routes
	app.POST("/audit", auditHdlr.AuditURL)

	app.Run()
}
