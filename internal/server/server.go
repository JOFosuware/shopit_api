package server

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	auth "github.com/jofosuware/go/shopit/internal/auth/delivery"
	order "github.com/jofosuware/go/shopit/internal/orders/delivery"
	payment "github.com/jofosuware/go/shopit/internal/payment/delivery"
	product "github.com/jofosuware/go/shopit/internal/products/delivery"

	"github.com/jofosuware/go/shopit/config"
	"github.com/jofosuware/go/shopit/pkg/logger"
)

var authHandlers *auth.AuthHandlers
var ordHandlers *order.OrderHandlers
var payHandlers *payment.PaymentHandler
var prodHandlers *product.ProdHandlers

// Serve holds the Server configuration
type Serve struct {
	cfg    *config.Config
	logger logger.Logger
	DB     *sql.DB
}

func NewServer(cfg *config.Config, logger logger.Logger, db *sql.DB) *Serve {
	return &Serve{
		cfg:    cfg,
		logger: logger,
		DB:     db,
	}
}

func (s *Serve) Run() error {
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%s", s.cfg.Server.Port),
		Handler:           s.Routes(),
		IdleTimeout:       30 * time.Second,
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      5 * time.Second,
	}

	s.logger.Infof("Starting Back end Serve in %s mode on port %s", s.cfg.Server.Mode, s.cfg.Server.Port)

	return srv.ListenAndServe()
}
