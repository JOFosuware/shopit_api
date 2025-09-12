package delivery_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jofosuware/go/shopit/internal/models"
	"github.com/jofosuware/go/shopit/internal/orders/delivery"
	mockOrder "github.com/jofosuware/go/shopit/internal/orders/mocks"
	mockLogger "github.com/jofosuware/go/shopit/pkg/logger/mock"
	"github.com/jofosuware/go/shopit/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const UserContextKey = utils.UserContextKey

func TestCreateOrder(t *testing.T) {
	logger := mockLogger.NewLogger(t)
	orderUC := mockOrder.NewOrderUC(t)

	o := delivery.NewOrderHandlers(logger, orderUC)

	t.Run("Order successfully created", func(t *testing.T) {
		// Prepare the payload matching the handler's anonymous struct.
		prodID := uuid.New().String()
		payload := struct {
			OrderItems []*struct {
				Product  string `json:"product"`
				Name     string `json:"name"`
				Price    int    `json:"price"`
				Image    string `json:"image"`
				Stock    int    `json:"stock"`
				Quantity int    `json:"quantity"`
			} `json:"orderItems"`
			ShippingInfo *struct {
				Address    string `json:"address"`
				City       string `json:"city"`
				PhoneNo    string `json:"phoneNo"`
				PostalCode string `json:"postalCode"`
				Country    string `json:"country"`
			} `json:"shippingInfo"`
			ItemsPrice    string `json:"itemsPrice"`
			ShippingPrice int    `json:"shippingPrice"`
			TaxPrice      int    `json:"taxPrice"`
			TotalPrice    string `json:"totalPrice"`
			PaymentInfo   *struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"paymentInfo"`
		}{
			OrderItems: []*struct {
				Product  string `json:"product"`
				Name     string `json:"name"`
				Price    int    `json:"price"`
				Image    string `json:"image"`
				Stock    int    `json:"stock"`
				Quantity int    `json:"quantity"`
			}{
				{
					Product:  prodID,
					Name:     "Test Product",
					Price:    100,
					Image:    "http://example.com/image.png",
					Stock:    10,
					Quantity: 1,
				},
			},
			ShippingInfo: &struct {
				Address    string `json:"address"`
				City       string `json:"city"`
				PhoneNo    string `json:"phoneNo"`
				PostalCode string `json:"postalCode"`
				Country    string `json:"country"`
			}{
				Address:    "123 Test Street",
				City:       "Test City",
				PhoneNo:    "1234567890",
				PostalCode: "00000",
				Country:    "TestLand",
			},
			ItemsPrice:    "100",
			ShippingPrice: 10,
			TaxPrice:      5,
			TotalPrice:    "115",
			PaymentInfo: &struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			}{
				ID:     "pay1",
				Status: "pending",
			},
		}

		jsonOrder, err := json.Marshal(payload)
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPost, "/orders", bytes.NewBuffer(jsonOrder))
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		// Set a valid user in context.
		user := models.User{ID: uuid.New()}
		ctx := context.WithValue(req.Context(), UserContextKey, &user)
		req = req.WithContext(ctx)

		// Expect the use case CreateOrder to be invoked.
		// (Using mock.Anything here for simplicity; you can use a more precise matcher if needed.)
		orderUC.On("CreateOrder", mock.Anything).Return(&models.Order{}, nil)

		o.CreateOrder(rr, req)

		got := rr.Code
		want := http.StatusOK

		assert.Equal(t, want, got)
	})
}

func TestGetSingleOrder(t *testing.T) {
	logger := mockLogger.NewLogger(t)
	orderUC := mockOrder.NewOrderUC(t)

	o := delivery.NewOrderHandlers(logger, orderUC)

	t.Run("Order successfully retrieved", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, "/orders/id", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		id := uuid.New()

		// Create a chi router and set the URL param
		rCtx := chi.NewRouteContext()
		rCtx.URLParams.Add("id", id.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rCtx))

		orderUC.On("GetSingleOrder", id).Return(&models.Order{}, nil)

		o.GetSingleOrder(rr, req)

		got := rr.Code
		want := http.StatusOK

		assert.Equal(t, want, got)
	})
}

func TestGetUserOrders(t *testing.T) {
	logger := mockLogger.NewLogger(t)
	orderUC := mockOrder.NewOrderUC(t)

	o := delivery.NewOrderHandlers(logger, orderUC)

	t.Run("Orders successfully retrieved", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, "/orders/user", nil)
		require.NoError(t, err)

		user := models.User{ID: uuid.New()}

		rr := httptest.NewRecorder()

		// mock session
		ctx := context.WithValue(req.Context(), UserContextKey, &user)
		req = req.WithContext(ctx)

		orderUC.On("GetUserOrders", user.ID).Return([]*models.Order{}, nil)

		o.GetUserOrders(rr, req)

		got := rr.Code
		want := http.StatusOK

		assert.Equal(t, want, got)
	})
}

func TestGetAllOrders(t *testing.T) {
	logger := mockLogger.NewLogger(t)
	orderUC := mockOrder.NewOrderUC(t)

	o := delivery.NewOrderHandlers(logger, orderUC)

	t.Run("All orders are successfully fetched", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, "/orders", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		orderUC.On("GetAllOrders").Return([]*models.Order{}, nil)

		o.GetAllOrders(rr, req)

		got := rr.Code
		want := http.StatusOK

		assert.Equal(t, want, got)
	})
}

func TestUpdateOrder(t *testing.T) {
	logger := mockLogger.NewLogger(t)
	orderUC := mockOrder.NewOrderUC(t)

	o := delivery.NewOrderHandlers(logger, orderUC)

	t.Run("Order is successfully updated", func(t *testing.T) {
		// Build multipart form data with the new status.
		formData := url.Values{
			"status": {"Delivered"},
		}
		payload, ct, err := utils.CreateMultipartForm(formData)
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPatch, "/order/update", payload)
		require.NoError(t, err)
		req.Header.Set("Content-Type", ct)

		rr := httptest.NewRecorder()

		// Set the chi route context to supply the order id from URL param.
		id := uuid.New()
		rCtx := chi.NewRouteContext()
		rCtx.URLParams.Add("id", id.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rCtx))

		// Create a fake order that is currently 'Processing'
		ord := models.Order{
			OrderItems: []*models.Item{
				{
					Quantity:  5,
					ProductID: uuid.New(),
				},
			},
			OrderStatus: "Processing",
		}

		// Expect GetSingleOrder to be called with the order id.
		orderUC.On("GetSingleOrder", id).Return(&ord, nil)

		// Expect UpdateStock to be called for each item.
		orderUC.On("UpdateStock", ord.OrderItems[0].ProductID, ord.OrderItems[0].Quantity).Return(nil)

		// For UpdateOrder, we expect that the order status is updated to "Delivered" and DeliveredAt is set.
		orderUC.
			On("UpdateOrder", mock.MatchedBy(func(updated models.Order) bool {
				// Check that the status is updated and DeliveredAt is non-zero.
				return updated.OrderStatus == "Delivered" && !updated.DeliveredAt.IsZero()
			})).
			Return(nil)

		// Call the handler.
		o.UpdateOrder(rr, req)

		// Assert that the response code is 200.
		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

func TestDeleteOrder(t *testing.T) {
	logger := mockLogger.NewLogger(t)
	orderUC := mockOrder.NewOrderUC(t)

	o := delivery.NewOrderHandlers(logger, orderUC)

	t.Run("Order is successfully deleted", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodDelete, "order/delete/id", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		id := uuid.New()

		// Create a chi router and set the URL param
		rCtx := chi.NewRouteContext()
		rCtx.URLParams.Add("id", id.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rCtx))

		orderUC.On("DeleteOrder", id).Return(nil)

		o.DeleteOrder(rr, req)

		got := rr.Code
		want := http.StatusOK

		assert.Equal(t, want, got)
	})
}
