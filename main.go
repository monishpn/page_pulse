package main

import (
	"gofr.dev/pkg/gofr"

	"github.com/monishpn/page_pulse/handler"
	"github.com/monishpn/page_pulse/service"
)

func main() {
	// initialize gofr object
	app := gofr.New()

	auditSvc := service.New()
	auditHdlr := handler.New(auditSvc)
	// register route greet
	app.POST("/audit", auditHdlr.AuditURL)

	// Runs the server, it will listen on the default port 8000.
	// it can be over-ridden through configs
	app.Run()
}
