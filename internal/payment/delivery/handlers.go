// Package delivery provides HTTP handlers for payment endpoints.
//
// It wires handler methods for processing payments and exposing payment-related
// configuration to callers.
package delivery

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jofosuware/go/shopit/config"
	"github.com/jofosuware/go/shopit/internal/models"
	"github.com/jofosuware/go/shopit/internal/orders"
	"github.com/jofosuware/go/shopit/pkg/logger"
	"github.com/jofosuware/go/shopit/pkg/utils"
)

// PaymentHandler provides HTTP handler methods for payment endpoints.
type PaymentHandler struct {
	cfg      *config.Config
	logger   logger.Logger
	payments PaymentIntentCreator
}

// PaymentIntentCreator creates a payment intent for a user's catalog-priced items.
type PaymentIntentCreator interface {
	CreatePaymentIntent(ctx context.Context, userID uuid.UUID, items []*models.Item) (string, string, error)
}

type createPaymentRequest struct {
	OrderItems []paymentItemRequest `json:"orderItems"`
}

type paymentItemRequest struct {
	Product  string `json:"product"`
	Quantity int    `json:"quantity"`
}

// NewPaymentHandler returns a new PaymentHandler.
func NewPaymentHandler(cfg *config.Config, logger logger.Logger, payments PaymentIntentCreator) *PaymentHandler {
	return &PaymentHandler{
		cfg:      cfg,
		logger:   logger,
		payments: payments,
	}
}

// ProcessPayment creates a PaymentIntent and returns its ID and client secret.
// Endpoint: POST /api/v1/payment/process
// Expects product IDs and quantities; the total is calculated from catalog data.
func (h *PaymentHandler) ProcessPayment(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(utils.UserContextKey).(*models.User)
	if !ok || user == nil {
		h.writeJSON(w, http.StatusUnauthorized, models.Response{
			Success: false, Message: "invalid authentication credentials",
		})
		return
	}
	if h.payments == nil {
		h.writeJSON(w, http.StatusInternalServerError, models.Response{
			Success: false, Message: "payment service is unavailable",
		})
		return
	}
	var request createPaymentRequest
	if err := utils.ReadJSON(w, r, &request); err != nil {
		h.writeBadRequest(w, "reading payment request", err)
		return
	}
	if len(request.OrderItems) == 0 {
		h.writeBadRequest(w, "validating payment request", errors.New("at least one order item is required"))
		return
	}
	if len(request.OrderItems) > orders.MaxOrderItems {
		h.writeBadRequest(w, "validating payment request", errors.New("order contains too many items"))
		return
	}
	items := make([]*models.Item, 0, len(request.OrderItems))
	for _, input := range request.OrderItems {
		productID, parseErr := uuid.Parse(input.Product)
		if parseErr != nil || input.Quantity <= 0 {
			h.writeBadRequest(w, "validating payment items", errors.New("invalid product ID or quantity"))
			return
		}
		items = append(items, &models.Item{ProductID: productID, Quantity: input.Quantity})
	}
	paymentID, clientSecret, err := h.payments.CreatePaymentIntent(r.Context(), user.ID, items)
	if err != nil {
		status, message := http.StatusInternalServerError, "payment could not be initialized"
		switch {
		case errors.Is(err, orders.ErrInvalidOrderRequest):
			status, message = http.StatusBadRequest, err.Error()
		case errors.Is(err, orders.ErrProductNotFound):
			status, message = http.StatusNotFound, "product not found"
		case errors.Is(err, orders.ErrInsufficientStock):
			status, message = http.StatusConflict, "insufficient product stock"
		}
		h.writeJSON(w, status, models.Response{Success: false, Message: message})
		if status == http.StatusInternalServerError && h.logger != nil {
			h.logger.Errorf("creating payment intent: %v", err)
		}
		return
	}
	if paymentID == "" || clientSecret == "" {
		h.writeJSON(w, http.StatusInternalServerError, models.Response{
			Success: false, Message: "payment could not be initialized",
		})
		if h.logger != nil {
			h.logger.Errorf("creating payment intent returned an incomplete result")
		}
		return
	}

	jsonRes := struct {
		Success      bool   `json:"success"`
		PaymentID    string `json:"payment_id"`
		ClientSecret string `json:"client_secret"`
	}{
		Success:      true,
		PaymentID:    paymentID,
		ClientSecret: clientSecret,
	}

	h.writeJSON(w, http.StatusOK, jsonRes)
}

// SendStripeApi returns the Stripe API key for the frontend to initialize Stripe.
// Endpoint: GET /api/v1/payment/stripeapi
func (h *PaymentHandler) SendStripeApi(w http.ResponseWriter, r *http.Request) {
	if h.cfg == nil || h.cfg.Stripe.Key == "" {
		h.writeJSON(w, http.StatusInternalServerError, models.Response{
			Success: false, Message: "payment service is unavailable",
		})
		if h.logger != nil {
			h.logger.Errorf("sending Stripe API key: publishable key is not configured")
		}
		return
	}
	jsonRes := struct {
		StripeApiKey string `json:"stripeApiKey"`
	}{
		StripeApiKey: h.cfg.Stripe.Key,
	}

	h.writeJSON(w, http.StatusOK, jsonRes)
}

func (h *PaymentHandler) writeJSON(w http.ResponseWriter, status int, payload any) {
	if err := utils.WriteJSON(w, status, payload); err != nil && h.logger != nil {
		h.logger.Errorf("writing payment response: %v", err)
	}
}

func (h *PaymentHandler) writeBadRequest(w http.ResponseWriter, operation string, err error) {
	if h.logger != nil {
		h.logger.Errorf("%s: %v", operation, err)
	}
	h.writeJSON(w, http.StatusBadRequest, models.Response{
		Success: false, Message: "invalid request",
	})
}
