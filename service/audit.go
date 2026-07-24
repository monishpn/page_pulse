package service

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/monishpn/page_pulse/models"
	"gofr.dev/pkg/gofr"
	"golang.org/x/net/html"
)

const (
	requestTimeout = 5 * time.Second
	maxRedirects   = 10
)

type service struct{}

func New() *service {
	return &service{}
}

func (s *service) AuditURL(ctx *gofr.Context, target string) (*models.AuditResponse, *models.CustomError) {
	var redirectCount int

	client := &http.Client{
		Timeout: requestTimeout,
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			redirectCount = len(via)
			if len(via) >= maxRedirects {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}

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

	return &models.AuditResponse{
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
		RedirectCount:   redirectCount,
		AuditedAt:       time.Now().UTC().Format(time.RFC3339),
	}, nil
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
