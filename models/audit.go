package models

type AuditResponse struct {
	URL             string `json:"url"`
	FinalURL        string `json:"finalUrl"`
	StatusCode      int    `json:"statusCode"`
	ResponseTimeMS  int64  `json:"responseTimeMs"`
	PageTitle       string `json:"pageTitle"`
	MetaDescription string `json:"metaDescription"`
	H1              string `json:"h1"`
	ContentType     string `json:"contentType"`
	ContentLength   int64  `json:"contentLength"`
	Server          string `json:"server"`
	HTTPS           bool   `json:"https"`
	RedirectCount   int    `json:"redirectCount"`
	AuditedAt       string `json:"auditedAt"`
}
