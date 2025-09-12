package delivery

import (
	"errors"
	"net/http"

	"github.com/jofosuware/go/shopit/config"
	"github.com/jofosuware/go/shopit/pkg/card"
	"github.com/jofosuware/go/shopit/pkg/logger"
	"github.com/jofosuware/go/shopit/pkg/utils"
)

// PaymentHandler is a payment's handler type
type PaymentHandler struct {
	cfg    *config.Config
	logger logger.Logger
	card   card.Carder
}

// NewPaymentHandler is the constructor for PaymentHandler
func NewPaymentHandler(cfg *config.Config, logger logger.Logger, card card.Carder) *PaymentHandler {
	return &PaymentHandler{
		cfg:    cfg,
		logger: logger,
		card:   card,
	}
}

// ProcessPayment process stripe payments   =>  /api/v1/payment/process
func (h *PaymentHandler) ProcessPayment(w http.ResponseWriter, r *http.Request) {
	type payment struct {
		Amount int `json:"amount"`
	}

	var p payment

	err := utils.ReadJSON(w, r, &p)
	if err != nil {
		_ = utils.BadRequest(w, r, errors.New("invalid json"))
		h.logger.Errorf("error reading json: %v", err)
		return
	}

	pi, _, err := h.card.CreatePaymentIntent("usd", p.Amount)
	if err != nil {
		_ = utils.BadRequest(w, r, errors.New("error charging card"))
		h.logger.Errorf("error creating payment intent: %v", err)
		return
	}

	jsonRes := struct {
		Success      bool   `json:"success"`
		ClientSecret string `json:"client_secret"`
	}{
		Success:      true,
		ClientSecret: pi.ClientSecret,
	}

	_ = utils.WriteJSON(w, http.StatusOK, jsonRes)
}

// SendStripeApi send stripe API Key   =>   /api/v1/payment/stripeapi
func (h *PaymentHandler) SendStripeApi(w http.ResponseWriter, r *http.Request) {
	jsonRes := struct {
		StripeApiKey string `json:"stripeApiKey"`
	}{
		StripeApiKey: h.cfg.Stripe.Key,
	}

	_ = utils.WriteJSON(w, http.StatusOK, jsonRes)
}
