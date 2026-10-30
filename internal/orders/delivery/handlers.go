// Package delivery provides HTTP handlers for order endpoints.
//
// It wires handler methods for creating, retrieving, updating, and deleting
// orders, and for listing orders for users and admins.
package delivery

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jofosuware/go/shopit/internal/models"
	"github.com/jofosuware/go/shopit/internal/orders"
	"github.com/jofosuware/go/shopit/pkg/logger"
	"github.com/jofosuware/go/shopit/pkg/utils"
)

// UserContextKey is the request context key used to store the authenticated user.
const UserContextKey = utils.UserContextKey

type createOrderRequest struct {
	OrderItems []struct {
		Product  string `json:"product"`
		Quantity int    `json:"quantity"`
	} `json:"orderItems"`
	ShippingInfo *struct {
		Address    string `json:"address"`
		City       string `json:"city"`
		PhoneNo    string `json:"phoneNo"`
		PostalCode string `json:"postalCode"`
		Country    string `json:"country"`
	} `json:"shippingInfo"`
	PaymentInfo *struct {
		ID string `json:"id"`
	} `json:"paymentInfo"`
}

// OrderHandlers provides HTTP handler methods for order endpoints.
type OrderHandlers struct {
	logger   logger.Logger
	ordersUC orders.OrderUC
}

// NewOrderHandlers returns a new OrderHandlers with the provided logger and usecase.
func NewOrderHandlers(logger logger.Logger, ordersUC orders.OrderUC) *OrderHandlers {
	return &OrderHandlers{
		logger:   logger,
		ordersUC: ordersUC,
	}
}

// CreateOrder creates a new order.
// Endpoint: POST /api/v1/orders/new
// Expects JSON body describing order items, shipping, and payment.
// @Summary Create an order
// @Tags Orders
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param order body createOrderRequest true "Order payload"
// @Success 200 {object} models.OrderResponse
// @Failure 400,401,402,404,409,500 {object} models.Response
// @Router /orders/new [post]
func (h *OrderHandlers) CreateOrder(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(UserContextKey).(*models.User)
	if !ok || user == nil {
		h.writeJSON(w, http.StatusUnauthorized, models.Response{Success: false, Message: "invalid authentication credentials"})
		return
	}

	var request createOrderRequest
	if err := utils.ReadJSON(w, r, &request); err != nil {
		h.writeBadRequest(w, "reading order payload", err)
		return
	}
	if len(request.OrderItems) == 0 || request.ShippingInfo == nil || request.PaymentInfo == nil {
		h.writeBadRequest(w, "validating order payload", errors.New("order items, shipping info, and payment info are required"))
		return
	}
	if len(request.OrderItems) > orders.MaxOrderItems {
		h.writeBadRequest(w, "validating order payload", errors.New("order contains too many items"))
		return
	}
	if strings.TrimSpace(request.PaymentInfo.ID) == "" ||
		strings.TrimSpace(request.ShippingInfo.Address) == "" ||
		strings.TrimSpace(request.ShippingInfo.City) == "" ||
		strings.TrimSpace(request.ShippingInfo.PhoneNo) == "" ||
		strings.TrimSpace(request.ShippingInfo.PostalCode) == "" ||
		strings.TrimSpace(request.ShippingInfo.Country) == "" {
		h.writeBadRequest(w, "validating order payload", errors.New("payment ID and complete shipping details are required"))
		return
	}

	order := models.Order{
		OrderItems:   make([]*models.Item, 0, len(request.OrderItems)),
		ShippingInfo: models.Shipping{Address: request.ShippingInfo.Address, City: request.ShippingInfo.City, PhoneNo: request.ShippingInfo.PhoneNo, PostalCode: request.ShippingInfo.PostalCode, Country: request.ShippingInfo.Country},
		PaymentInfo:  models.Payment{ID: strings.TrimSpace(request.PaymentInfo.ID)},
		UserID:       user.ID,
	}
	for _, input := range request.OrderItems {
		productID, err := uuid.Parse(input.Product)
		if err != nil || input.Quantity <= 0 {
			h.writeBadRequest(w, "validating order items", errors.New("invalid product ID or quantity"))
			return
		}
		order.OrderItems = append(order.OrderItems, &models.Item{
			ProductID: productID,
			Quantity:  input.Quantity,
		})
	}

	createdOrder, err := h.ordersUC.CreateOrder(r.Context(), order)
	if err != nil {
		h.writeOrderError(w, err)
		h.logErrorf("creating order: %v", err)
		return
	}
	if createdOrder == nil {
		h.writeOrderError(w, errors.New("order use case returned an empty result"))
		return
	}

	jr := models.OrderResponse{
		Success: true,
		Order:   *createdOrder,
	}

	h.writeJSON(w, http.StatusOK, jr)
}

// GetSingleOrder returns an order by its ID.
// Endpoint: GET /api/v1/orders/{id}
// @Summary Get an order
// @Tags Orders
// @Produce json
// @Security BearerAuth
// @Param id path string true "Order ID" format(uuid)
// @Success 200 {object} models.OrderResponse
// @Failure 400,401,403,404,500 {object} models.Response
// @Router /orders/{id} [get]
func (h *OrderHandlers) GetSingleOrder(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(UserContextKey).(*models.User)
	if !ok || user == nil {
		h.writeJSON(w, http.StatusUnauthorized, models.Response{Success: false, Message: "invalid authentication credentials"})
		return
	}
	id := chi.URLParam(r, "id")

	parsedId, err := uuid.Parse(id)
	if err != nil {
		h.writeBadRequest(w, "parsing order ID", err)
		return
	}

	order, err := h.ordersUC.GetSingleOrder(r.Context(), parsedId)
	if err != nil {
		h.writeOrderError(w, err)
		h.logErrorf("getting order: %v", err)
		return
	}
	if order == nil {
		h.writeOrderError(w, orders.ErrOrderNotFound)
		return
	}
	if user.Role != "admin" && order.UserID != user.ID {
		h.writeJSON(w, http.StatusForbidden, models.Response{Success: false, Message: "forbidden"})
		return
	}

	jr := models.OrderResponse{
		Success: true,
		Order:   *order,
	}

	h.writeJSON(w, http.StatusOK, jr)
}

// GetUserOrders returns orders for the currently authenticated user.
// Endpoint: GET /api/v1/orders/me
// @Summary List current user's orders
// @Tags Orders
// @Produce json
// @Security BearerAuth
// @Param page query integer false "Page" minimum(1) default(1)
// @Param pageSize query integer false "Page size" minimum(1) maximum(100) default(20)
// @Success 200 {object} object
// @Failure 400,401,500 {object} models.Response
// @Router /orders/me [get]
func (h *OrderHandlers) GetUserOrders(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(UserContextKey).(*models.User)
	if !ok || user == nil {
		h.writeJSON(w, http.StatusUnauthorized, models.Response{Success: false, Message: "invalid authentication credentials"})
		return
	}

	page, pageSize, err := parsePagination(r)
	if err != nil {
		h.writeOrderError(w, err)
		return
	}
	ords, totalCount, totalAmount, err := h.ordersUC.GetUserOrders(r.Context(), user.ID, page, pageSize)
	if err != nil {
		h.writeOrderError(w, err)
		h.logErrorf("getting user orders: %v", err)
		return
	}

	jr := struct {
		Success     bool            `json:"success"`
		Orders      []*models.Order `json:"orders"`
		Page        int             `json:"page"`
		PageSize    int             `json:"pageSize"`
		TotalCount  int             `json:"totalCount"`
		TotalAmount int64           `json:"totalAmount"`
	}{
		Success: true, Orders: ords, Page: page, PageSize: pageSize,
		TotalCount: totalCount, TotalAmount: totalAmount,
	}

	h.writeJSON(w, http.StatusOK, jr)
}

// GetAllOrders returns all orders (admin).
// Endpoint: GET /api/v1/orders/admin/orders
// @Summary List all orders
// @Tags Orders
// @Produce json
// @Security BearerAuth
// @Param page query integer false "Page" minimum(1) default(1)
// @Param pageSize query integer false "Page size" minimum(1) maximum(100) default(20)
// @Success 200 {object} object
// @Failure 400,401,403,500 {object} models.Response
// @Router /orders/admin/orders [get]
func (h *OrderHandlers) GetAllOrders(w http.ResponseWriter, r *http.Request) {
	page, pageSize, err := parsePagination(r)
	if err != nil {
		h.writeOrderError(w, err)
		return
	}
	ords, totalCount, totalAmount, err := h.ordersUC.GetAllOrders(r.Context(), page, pageSize)
	if err != nil {
		h.writeOrderError(w, err)
		h.logErrorf("getting orders: %v", err)
		return
	}

	jr := struct {
		Success     bool            `json:"success"`
		TotalAmount int64           `json:"totalAmount"`
		TotalCount  int             `json:"totalCount"`
		Page        int             `json:"page"`
		PageSize    int             `json:"pageSize"`
		Orders      []*models.Order `json:"orders"`
	}{
		Success: true, TotalCount: totalCount, Page: page, PageSize: pageSize,
		TotalAmount: totalAmount,
		Orders:      ords,
	}

	h.writeJSON(w, http.StatusOK, jr)
}

// UpdateOrder updates an order's status (admin).
// Endpoint: PUT /api/v1/orders/admin/order/{id}
// Expects form data: status.
// @Summary Update order status
// @Tags Orders
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param id path string true "Order ID" format(uuid)
// @Param status formData string true "Status" Enums(Processing,Delivered)
// @Success 200 {object} models.Response
// @Failure 400,401,403,404,409,500 {object} models.Response
// @Router /orders/admin/order/{id} [put]
func (h *OrderHandlers) UpdateOrder(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	parsedId, err := uuid.Parse(id)
	if err != nil {
		h.writeBadRequest(w, "parsing order ID", err)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	err = r.ParseMultipartForm(1 << 20)
	if err != nil {
		h.writeBadRequest(w, "parsing order status form", err)
		return
	}

	status := r.FormValue("status")
	if status != "Processing" && status != "Delivered" {
		h.writeOrderError(w, orders.ErrInvalidOrderStatus)
		return
	}

	err = h.ordersUC.UpdateOrderStatus(r.Context(), parsedId, status)
	if err != nil {
		h.writeOrderError(w, err)
		h.logErrorf("updating order: %v", err)
		return
	}

	jsonRes := struct {
		Success bool `json:"success"`
	}{
		Success: true,
	}

	h.writeJSON(w, http.StatusOK, jsonRes)
}

// DeleteOrder deletes an order (admin).
// Endpoint: DELETE /api/v1/orders/admin/order/{id}
// @Summary Delete an order
// @Tags Orders
// @Produce json
// @Security BearerAuth
// @Param id path string true "Order ID" format(uuid)
// @Success 200 {object} models.Response
// @Failure 400,401,403,404,500 {object} models.Response
// @Router /orders/admin/order/{id} [delete]
func (h *OrderHandlers) DeleteOrder(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	parsedId, err := uuid.Parse(id)
	if err != nil {
		h.writeBadRequest(w, "parsing order ID", err)
		return
	}

	err = h.ordersUC.DeleteOrder(r.Context(), parsedId)
	if err != nil {
		h.writeOrderError(w, err)
		h.logErrorf("deleting order: %v", err)
		return
	}

	jsonRes := struct {
		Success bool `json:"success"`
	}{
		Success: true,
	}

	h.writeJSON(w, http.StatusOK, jsonRes)
}

func (h *OrderHandlers) writeOrderError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	message := "internal server error"
	switch {
	case errors.Is(err, orders.ErrOrderNotFound):
		status, message = http.StatusNotFound, "order not found"
	case errors.Is(err, orders.ErrOrderAlreadyDelivered), errors.Is(err, orders.ErrInsufficientStock):
		status, message = http.StatusConflict, err.Error()
	case errors.Is(err, orders.ErrInvalidOrderStatus):
		status, message = http.StatusBadRequest, err.Error()
	case errors.Is(err, orders.ErrInvalidOrderRequest):
		status, message = http.StatusBadRequest, err.Error()
	case errors.Is(err, orders.ErrInvalidPagination):
		status, message = http.StatusBadRequest, err.Error()
	case errors.Is(err, orders.ErrInvalidPayment):
		status, message = http.StatusPaymentRequired, err.Error()
	case errors.Is(err, orders.ErrPaymentAlreadyUsed):
		status, message = http.StatusConflict, err.Error()
	case errors.Is(err, orders.ErrPaymentRefundFailed), errors.Is(err, orders.ErrPaymentUseUnknown):
		status, message = http.StatusInternalServerError, "payment requires reconciliation"
	case errors.Is(err, orders.ErrProductNotFound):
		status, message = http.StatusNotFound, "product not found"
	case errors.Is(err, orders.ErrOrderPriceChanged):
		status, message = http.StatusConflict, err.Error()
	}
	h.writeJSON(w, status, models.Response{Success: false, Message: message})
}

func parsePagination(r *http.Request) (int, int, error) {
	const defaultPage, defaultPageSize = 1, 20
	page, pageSize := defaultPage, defaultPageSize
	if raw := r.URL.Query().Get("page"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			return 0, 0, orders.ErrInvalidPagination
		}
		page = parsed
	}
	if raw := r.URL.Query().Get("pageSize"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 100 {
			return 0, 0, orders.ErrInvalidPagination
		}
		pageSize = parsed
	}
	return page, pageSize, nil
}

func (h *OrderHandlers) writeJSON(w http.ResponseWriter, status int, payload any) {
	if writeErr := utils.WriteJSON(w, status, payload); writeErr != nil {
		h.logErrorf("writing order response: %v", writeErr)
	}
}

func (h *OrderHandlers) writeBadRequest(w http.ResponseWriter, operation string, err error) {
	h.logErrorf("%s: %v", operation, err)
	h.writeJSON(w, http.StatusBadRequest, models.Response{Success: false, Message: "invalid request"})
}

func (h *OrderHandlers) logErrorf(format string, args ...any) {
	if h.logger != nil {
		h.logger.Errorf(format, args...)
	}
}
