package handler

import (
	"github.com/monishpn/page_pulse/models"
	"gofr.dev/pkg/gofr"
	gofrHttp "gofr.dev/pkg/gofr/http"
)

type AuditService interface {
	AuditURL(ctx *gofr.Context, url string) (*models.AuditResponse, *models.CustomError)
}

type handler struct {
	Service AuditService
}

func New(svc AuditService) *handler {
	return &handler{Service: svc}
}

func (h *handler) AuditURL(ctx *gofr.Context) (any, error) {
	var body struct {
		URL string `json:"url"`
	}

	if err := ctx.Bind(&body); err != nil {
		return nil, gofrHttp.ErrorInvalidParam{Params: []string{"url"}}
	}

	return h.Service.AuditURL(ctx, body.URL)
}
