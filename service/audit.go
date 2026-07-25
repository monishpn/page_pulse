package service

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/monishpn/page_pulse/models"
	"gofr.dev/pkg/gofr"
	"golang.org/x/net/html"
)

type CacheStore interface {
	GetAudit(ctx *gofr.Context, key string) (*models.AuditResponse, error)
	PutAudit(ctx *gofr.Context, a *models.AuditResponse) error
	CheckAudit(ctx *gofr.Context, key string) (bool, error)
}

type service struct {
	requestTimeout time.Duration
	cacheStore     CacheStore
}

func New(requestTimeout string, cacheStore CacheStore) *service {
	timeoutInt, err := strconv.Atoi(requestTimeout)
	if err != nil {
		// If timeout not in int, set it back to default
		timeoutInt = 30
	}

	return &service{
		requestTimeout: time.Duration(timeoutInt) * time.Second,
		cacheStore:     cacheStore,
	}

}

func (s *service) AuditURL(ctx *gofr.Context, target string) (*models.AuditResponse, *models.CustomError) {
	if exists, err := s.cacheStore.CheckAudit(ctx, target); err == nil && exists {
		if cached, err := s.cacheStore.GetAudit(ctx, target); err == nil && cached != nil {
			ctx.Logger.Infof("URL: %s extracted from Cache", target)
			return cached, nil
		}
	}

	client := &http.Client{Timeout: s.requestTimeout}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, http.NoBody)
	if err != nil {
		return nil, &models.CustomError{Message: "invalid url: " + err.Error(), Code: http.StatusBadRequest}
	}

	start := time.Now()

	resp, err := client.Do(req)
	if err != nil {
		if uerr, ok := err.(*url.Error); ok && uerr.Timeout() {
			return nil, &models.CustomError{Message: "request timed out: " + err.Error(), Code: http.StatusGatewayTimeout}
		}
		return nil, &models.CustomError{Message: "failed to reach url: " + err.Error(), Code: http.StatusBadGateway}
	}
	defer resp.Body.Close()

	responseTimeMS := time.Since(start).Milliseconds()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &models.CustomError{Message: "failed to read response body: " + err.Error(), Code: http.StatusBadGateway}
	}

	title, metaDescription, h1 := parseHTML(body)

	audit := &models.AuditResponse{
		URL:             target,
		FinalURL:        resp.Request.URL.String(),
		StatusCode:      resp.StatusCode,
		ResponseTimeMS:  responseTimeMS,
		PageTitle:       title,
		MetaDescription: metaDescription,
		H1:              h1,
		ContentType:     resp.Header.Get("Content-Type"),
		ContentLength:   int64(len(body)),
		Server:          resp.Header.Get("Server"),
		HTTPS:           resp.Request.URL.Scheme == "https",
		AuditedAt:       time.Now().UTC().Format(time.RFC3339),
	}

	err = s.cacheStore.PutAudit(ctx, audit)
	if err != nil {
		ctx.Logger.Errorf("failed to put audit data of URL: %v to cache. Error: %v", target, err)
	}

	return audit, nil
}

// parseHTML walks the token stream once, picking out the page title, the
// meta description, and the first h1 — the only fields FR-4 requires.
func parseHTML(body []byte) (title, metaDescription, h1 string) {
	tokenizer := html.NewTokenizer(bytes.NewReader(body))

	for {
		switch tokenizer.Next() {
		case html.ErrorToken:
			return title, metaDescription, h1

		case html.StartTagToken, html.SelfClosingTagToken:
			token := tokenizer.Token()

			switch token.Data {
			case "title":
				if title == "" && tokenizer.Next() == html.TextToken {
					title = strings.TrimSpace(tokenizer.Token().Data)
				}
			case "h1":
				if h1 == "" && tokenizer.Next() == html.TextToken {
					h1 = strings.TrimSpace(tokenizer.Token().Data)
				}
			case "meta":
				if metaDescription == "" {
					metaDescription = extractMetaDescription(token.Attr)
				}
			}
		}
	}
}

func extractMetaDescription(attrs []html.Attribute) string {
	var name, content string

	for _, attr := range attrs {
		switch strings.ToLower(attr.Key) {
		case "name":
			name = strings.ToLower(attr.Val)
		case "content":
			content = attr.Val
		}
	}

	if name == "description" {
		return strings.TrimSpace(content)
	}

	return ""
}
