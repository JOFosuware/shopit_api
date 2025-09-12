package delivery_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jofosuware/go/shopit/config"
	"github.com/jofosuware/go/shopit/internal/payment/delivery"
	mockCard "github.com/jofosuware/go/shopit/pkg/card/mocks"
	mockLogger "github.com/jofosuware/go/shopit/pkg/logger/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v72"
)

func TestProcessPayment(t *testing.T) {
	cfg := config.Config{}
	logger := mockLogger.NewLogger(t)
	carder := mockCard.NewCarder(t)

	h := delivery.NewPaymentHandler(&cfg, logger, carder)

	// Updated JSON payload: amount is an integer (5) instead of a string.
	jsonData := []byte(`{"amount": 5}`)
	t.Run("Payment is successfully processed", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPost, "/payment", bytes.NewBuffer(jsonData))
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		// Expect CreatePaymentIntent to be called with "usd" and amount 5.
		carder.On("CreatePaymentIntent", "usd", 5).Return(&stripe.PaymentIntent{ClientSecret: "test_secret"}, "", nil)

		h.ProcessPayment(rr, req)

		got := rr.Code
		want := http.StatusOK

		assert.Equal(t, want, got)
	})
}
