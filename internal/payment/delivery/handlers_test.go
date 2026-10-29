package delivery_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jofosuware/go/shopit/config"
	"github.com/jofosuware/go/shopit/internal/models"
	"github.com/jofosuware/go/shopit/internal/orders"
	"github.com/jofosuware/go/shopit/internal/orders/mocks"
	"github.com/jofosuware/go/shopit/internal/payment/delivery"
	mockLogger "github.com/jofosuware/go/shopit/pkg/logger/mock"
	"github.com/jofosuware/go/shopit/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestProcessPayment(t *testing.T) {
	cfg := config.Config{}
	logger := mockLogger.NewLogger(t)
	orderUC := mocks.NewOrderUC(t)
	h := delivery.NewPaymentHandler(&cfg, logger, orderUC)

	productID := uuid.New()
	jsonData, err := json.Marshal(map[string]any{
		"orderItems": []map[string]any{{"product": productID.String(), "quantity": 2}},
	})
	require.NoError(t, err)
	t.Run("Payment is successfully processed", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPost, "/payment", bytes.NewBuffer(jsonData))
		require.NoError(t, err)
		user := &models.User{ID: uuid.New()}
		req = req.WithContext(context.WithValue(req.Context(), utils.UserContextKey, user))

		rr := httptest.NewRecorder()

		orderUC.On("CreatePaymentIntent", req.Context(), user.ID, mock.MatchedBy(func(items []*models.Item) bool {
			return len(items) == 1 && items[0].ProductID == productID && items[0].Quantity == 2
		})).Return("pi_test", "test_secret", nil)

		h.ProcessPayment(rr, req)

		got := rr.Code
		want := http.StatusOK

		assert.Equal(t, want, got)
		var response struct {
			PaymentID    string `json:"payment_id"`
			ClientSecret string `json:"client_secret"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &response))
		assert.Equal(t, "pi_test", response.PaymentID)
		assert.Equal(t, "test_secret", response.ClientSecret)
	})
}

func TestProcessPaymentRejectsMissingAuthentication(t *testing.T) {
	handler := delivery.NewPaymentHandler(&config.Config{}, nil, mocks.NewOrderUC(t))
	request := httptest.NewRequest(http.MethodPost, "/payment", bytes.NewBufferString(`{"orderItems":[]}`))
	response := httptest.NewRecorder()

	handler.ProcessPayment(response, request)

	require.Equal(t, http.StatusUnauthorized, response.Code)
}

func TestProcessPaymentHandlesMissingUseCase(t *testing.T) {
	handler := delivery.NewPaymentHandler(&config.Config{}, nil, nil)
	request := httptest.NewRequest(http.MethodPost, "/payment", bytes.NewBufferString(`{}`))
	request = request.WithContext(context.WithValue(request.Context(), utils.UserContextKey, &models.User{ID: uuid.New()}))
	response := httptest.NewRecorder()

	handler.ProcessPayment(response, request)

	require.Equal(t, http.StatusInternalServerError, response.Code)
}

func TestProcessPaymentRejectsMalformedOrInvalidItems(t *testing.T) {
	testCases := []struct {
		name string
		body string
	}{
		{name: "malformed JSON", body: `{"orderItems":`},
		{name: "empty items", body: `{"orderItems":[]}`},
		{name: "too many items", body: tooManyPaymentItemsJSON()},
		{name: "invalid product ID", body: `{"orderItems":[{"product":"not-a-uuid","quantity":1}]}`},
		{name: "non-positive quantity", body: `{"orderItems":[{"product":"00000000-0000-0000-0000-000000000001","quantity":0}]}`},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			orderUC := mocks.NewOrderUC(t)
			handler := delivery.NewPaymentHandler(&config.Config{}, nil, orderUC)
			request := httptest.NewRequest(http.MethodPost, "/payment", bytes.NewBufferString(testCase.body))
			request = request.WithContext(context.WithValue(request.Context(), utils.UserContextKey, &models.User{ID: uuid.New()}))
			response := httptest.NewRecorder()

			handler.ProcessPayment(response, request)

			require.Equal(t, http.StatusBadRequest, response.Code)
			orderUC.AssertNotCalled(t, "CreatePaymentIntent", mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

func tooManyPaymentItemsJSON() string {
	item := `{"product":"00000000-0000-0000-0000-000000000001","quantity":1}`
	var body strings.Builder
	body.WriteString(`{"orderItems":[`)
	for i := 0; i < orders.MaxOrderItems+1; i++ {
		if i > 0 {
			body.WriteByte(',')
		}
		body.WriteString(item)
	}
	body.WriteString(`]}`)
	return body.String()
}

func TestProcessPaymentMapsCheckoutErrors(t *testing.T) {
	productID := uuid.New()
	body, err := json.Marshal(map[string]any{
		"orderItems": []map[string]any{{"product": productID.String(), "quantity": 1}},
	})
	require.NoError(t, err)

	testCases := []struct {
		name       string
		cause      error
		statusCode int
	}{
		{name: "unknown product", cause: orders.ErrProductNotFound, statusCode: http.StatusNotFound},
		{name: "out of stock", cause: orders.ErrInsufficientStock, statusCode: http.StatusConflict},
		{name: "invalid checkout", cause: orders.ErrInvalidOrderRequest, statusCode: http.StatusBadRequest},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			userID := uuid.New()
			orderUC := mocks.NewOrderUC(t)
			orderUC.On("CreatePaymentIntent", mock.Anything, userID, mock.Anything).
				Return("", "", testCase.cause).Once()
			handler := delivery.NewPaymentHandler(&config.Config{}, nil, orderUC)
			request := httptest.NewRequest(http.MethodPost, "/payment", bytes.NewReader(body))
			request = request.WithContext(context.WithValue(request.Context(), utils.UserContextKey, &models.User{ID: userID}))
			response := httptest.NewRecorder()

			handler.ProcessPayment(response, request)

			require.Equal(t, testCase.statusCode, response.Code)
		})
	}
}

func TestProcessPaymentRejectsIncompleteUseCaseResult(t *testing.T) {
	userID := uuid.New()
	orderUC := mocks.NewOrderUC(t)
	orderUC.On("CreatePaymentIntent", mock.Anything, userID, mock.Anything).
		Return("pi_created", "", nil).Once()
	handler := delivery.NewPaymentHandler(&config.Config{}, nil, orderUC)
	request := httptest.NewRequest(http.MethodPost, "/payment", bytes.NewBufferString(
		`{"orderItems":[{"product":"00000000-0000-0000-0000-000000000001","quantity":1}]}`,
	))
	request = request.WithContext(context.WithValue(request.Context(), utils.UserContextKey, &models.User{ID: userID}))
	response := httptest.NewRecorder()

	handler.ProcessPayment(response, request)

	require.Equal(t, http.StatusInternalServerError, response.Code)
}

func TestSendStripeAPIHandlesMissingConfiguration(t *testing.T) {
	handler := delivery.NewPaymentHandler(nil, nil, nil)
	response := httptest.NewRecorder()

	handler.SendStripeApi(response, httptest.NewRequest(http.MethodGet, "/payment/stripeapi", nil))

	require.Equal(t, http.StatusInternalServerError, response.Code)
}

func TestSendStripeAPIReturnsConfiguredPublishableKey(t *testing.T) {
	cfg := &config.Config{}
	cfg.Stripe.Key = "pk_test_example"
	handler := delivery.NewPaymentHandler(cfg, nil, nil)
	response := httptest.NewRecorder()

	handler.SendStripeApi(response, httptest.NewRequest(http.MethodGet, "/payment/stripeapi", nil))

	require.Equal(t, http.StatusOK, response.Code)
	var payload map[string]string
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &payload))
	require.Equal(t, map[string]string{"stripeApiKey": "pk_test_example"}, payload)
}
