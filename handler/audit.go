package handler

import (
	"net/http"

	"github.com/monishpn/page_pulse/models"
	"github.com/monishpn/page_pulse/validator"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/http/response"
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
		return nil, &models.CustomError{Message: "invalid request body", Code: http.StatusBadRequest}
	}

	if err := validator.ValidateURL(body.URL); err != nil {
		return nil, err
	}

	audit, err := h.Service.AuditURL(ctx, body.URL)
	if err != nil {
		return nil, err
	}

	return response.Raw{Data: audit}, nil
}
