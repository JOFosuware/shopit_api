package delivery_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	authRepository "github.com/jofosuware/go/shopit/internal/auth/repository"
	"github.com/jofosuware/go/shopit/internal/models"
	"github.com/jofosuware/go/shopit/internal/orders"
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
		orderUC.On("CreateOrder", mock.Anything, mock.MatchedBy(func(order models.Order) bool {
			return len(order.OrderItems) == 1 && order.OrderItems[0].Quantity == 1 &&
				order.OrderItems[0].ProductID.String() == prodID &&
				order.PaymentInfo.ID == "pay1" && order.PaymentInfo.Status == "" &&
				order.ItemPrice == 0 && order.TotalPrice == 0
		})).Return(&models.Order{}, nil)

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
		user := &models.User{ID: uuid.New()}
		ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rCtx)
		ctx = context.WithValue(ctx, UserContextKey, user)
		req = req.WithContext(ctx)

		orderUC.On("GetSingleOrder", mock.Anything, id).Return(&models.Order{UserID: user.ID}, nil)

		o.GetSingleOrder(rr, req)

		got := rr.Code
		want := http.StatusOK

		assert.Equal(t, want, got)
	})
}

func TestGetSingleOrderRejectsAnotherUsersOrder(t *testing.T) {
	logger := mockLogger.NewLogger(t)
	orderUC := mockOrder.NewOrderUC(t)
	handler := delivery.NewOrderHandlers(logger, orderUC)

	orderID := uuid.New()
	request, err := http.NewRequest(http.MethodGet, "/orders/"+orderID.String(), nil)
	require.NoError(t, err)
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("id", orderID.String())
	requestContext := context.WithValue(request.Context(), chi.RouteCtxKey, routeContext)
	requestContext = context.WithValue(requestContext, UserContextKey, &models.User{ID: uuid.New(), Role: "user"})
	request = request.WithContext(requestContext)
	orderUC.On("GetSingleOrder", mock.Anything, orderID).
		Return(&models.Order{OrderID: orderID, UserID: uuid.New()}, nil).Once()

	response := httptest.NewRecorder()
	handler.GetSingleOrder(response, request)

	require.Equal(t, http.StatusForbidden, response.Code)
	orderUC.AssertExpectations(t)
}

func TestCreateOrderRejectsEmptyItems(t *testing.T) {
	orderUC := mockOrder.NewOrderUC(t)
	handler := delivery.NewOrderHandlers(nil, orderUC)
	request, err := http.NewRequest(http.MethodPost, "/orders", bytes.NewBufferString(`{"orderItems":[],"shippingInfo":{},"paymentInfo":{}}`))
	require.NoError(t, err)
	request = request.WithContext(context.WithValue(request.Context(), UserContextKey, &models.User{ID: uuid.New()}))

	response := httptest.NewRecorder()
	handler.CreateOrder(response, request)

	require.Equal(t, http.StatusBadRequest, response.Code)
	orderUC.AssertNotCalled(t, "CreateOrder", mock.Anything, mock.Anything)
}

func TestCreateOrderRejectsTooManyItems(t *testing.T) {
	handler := delivery.NewOrderHandlers(nil, mockOrder.NewOrderUC(t))
	productID := uuid.NewString()
	item := `{"product":"` + productID + `","quantity":1}`
	var body bytes.Buffer
	body.WriteString(`{"orderItems":[`)
	for i := 0; i < orders.MaxOrderItems+1; i++ {
		if i > 0 {
			body.WriteByte(',')
		}
		body.WriteString(item)
	}
	body.WriteString(`],"shippingInfo":{"address":"A","city":"C","phoneNo":"P","postalCode":"Z","country":"X"},"paymentInfo":{"id":"pi"}}`)
	request := httptest.NewRequest(http.MethodPost, "/orders/new", &body)
	request = request.WithContext(context.WithValue(request.Context(), UserContextKey, &models.User{ID: uuid.New()}))
	response := httptest.NewRecorder()

	handler.CreateOrder(response, request)

	require.Equal(t, http.StatusBadRequest, response.Code)
}

func TestOrderRouterRestrictsAdminRoutes(t *testing.T) {
	db, sqlMock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	previousRepo := utils.Repo
	utils.Repo = authRepository.NewAuthRepository(db)
	t.Cleanup(func() { utils.Repo = previousRepo })

	token := "abcdefghijklmnopqrstuvwx12"
	userID := uuid.New()
	sqlMock.ExpectQuery(`(?s)select.*from\s+users u.*t\.token_hash`).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "name", "email", "role"}).
			AddRow(userID, "user", "user@example.test", "user"))

	orderUC := mockOrder.NewOrderUC(t)
	handler := delivery.NewOrderHandlers(mockLogger.NewLogger(t), orderUC)
	request := httptest.NewRequest(http.MethodGet, "/admin/orders", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()

	handler.OrderRouter().ServeHTTP(response, request)

	require.NoError(t, sqlMock.ExpectationsWereMet())
	require.Equal(t, http.StatusForbidden, response.Code)
	orderUC.AssertNotCalled(t, "GetAllOrders", mock.Anything)
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

		orderUC.On("GetUserOrders", mock.Anything, user.ID, 1, 20).
			Return([]*models.Order{}, 0, int64(0), nil)

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

		orderUC.On("GetAllOrders", mock.Anything, 1, 20).
			Return([]*models.Order{}, 0, int64(0), nil)

		o.GetAllOrders(rr, req)

		got := rr.Code
		want := http.StatusOK

		assert.Equal(t, want, got)
	})
}

func TestGetUserOrdersRejectsOutOfRangePageSize(t *testing.T) {
	orderUC := mockOrder.NewOrderUC(t)
	handler := delivery.NewOrderHandlers(mockLogger.NewLogger(t), orderUC)
	request := httptest.NewRequest(http.MethodGet, "/orders/me?pageSize=101", nil)
	request = request.WithContext(context.WithValue(request.Context(), UserContextKey, &models.User{ID: uuid.New()}))
	response := httptest.NewRecorder()

	handler.GetUserOrders(response, request)

	require.Equal(t, http.StatusBadRequest, response.Code)
	orderUC.AssertNotCalled(t, "GetUserOrders", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
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

		orderUC.On("UpdateOrderStatus", mock.Anything, id, "Delivered").Return(nil)

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

		orderUC.On("DeleteOrder", mock.Anything, id).Return(nil)

		o.DeleteOrder(rr, req)

		got := rr.Code
		want := http.StatusOK

		assert.Equal(t, want, got)
	})
}
