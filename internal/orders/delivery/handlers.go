package delivery

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jofosuware/go/shopit/internal/models"
	"github.com/jofosuware/go/shopit/internal/orders"
	"github.com/jofosuware/go/shopit/pkg/logger"
	"github.com/jofosuware/go/shopit/pkg/utils"
	"github.com/jofosuware/go/shopit/pkg/validator"
)

const UserContextKey = utils.UserContextKey

// OrderHandlers is the type struct for Order handler
type OrderHandlers struct {
	logger   logger.Logger
	ordersUC orders.OrderUC
}

// NewOrderHandlers is the constructor for OrderHandlers
func NewOrderHandlers(logger logger.Logger, ordersUC orders.OrderUC) *OrderHandlers {
	return &OrderHandlers{
		logger:   logger,
		ordersUC: ordersUC,
	}
}

// CreateOrder creates a new order   =>  /api/v1/orders/new
func (h *OrderHandlers) CreateOrder(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(UserContextKey).(*models.User)
	if !ok {
		_ = utils.BadRequest(w, r, errors.New("error getting user from session"))
		h.logger.Errorf("error getting user from session")
		return
	}

	ord := &models.Order{
		OrderItems:   []*models.Item{new(models.Item)},
		ShippingInfo: models.Shipping{},
		PaymentInfo:  models.Payment{},
	}

	 order := struct {
		OrderItems []*struct{
			Product string `json:"product"`
			Name   string `json:"name"`
			Price  int    `json:"price"`
			Image string `json:"image"`
			Stock int    `json:"stock"`
			Quantity int `json:"quantity"`
		} `json:"orderItems"`
		ShippingInfo *struct {
			Address string `json:"address"`
			City    string `json:"city"`
			PhoneNo string `json:"phoneNo"`
			PostalCode string `json:"postalCode"`
			Country string `json:"country"`
		} `json:"shippingInfo"`
		ItemsPrice string `json:"itemsPrice"`
		ShippingPrice int `json:"shippingPrice"`
		TaxPrice float64 `json:"taxPrice"`
		TotalPrice string `json:"totalPrice"`
		PaymentInfo *struct {
			ID string `json:"id"`
			Status string `json:"status"`
		} `json:"paymentInfo"`
	}{}

	if err := utils.ReadJSON(w, r, &order); err != nil {
		_ = utils.BadRequest(w, r, errors.New("bad request"))
		h.logger.Errorf("error parsing payload: %v", err)
		return
	}

	parsedId, err := uuid.Parse(order.OrderItems[0].Product)
	itemPrice, _ := strconv.ParseFloat(order.ItemsPrice, 64)
	totalPrice, _ := strconv.ParseFloat(order.TotalPrice, 64)
	if err != nil {
		_ = utils.BadRequest(w, r, errors.New("bad request"))
		h.logger.Errorf("error parsing payload: %v", err)
		return
	}

	ord.OrderItems[0].ProductID = parsedId
	ord.OrderItems[0].Name = order.OrderItems[0].Name
	ord.OrderItems[0].Price = order.OrderItems[0].Price
	ord.OrderItems[0].Quantity = order.OrderItems[0].Quantity
	ord.OrderItems[0].Image = order.OrderItems[0].Image
	ord.ShippingInfo.Address = order.ShippingInfo.Address
	ord.ShippingInfo.City = order.ShippingInfo.City
	ord.ShippingInfo.PhoneNo = order.ShippingInfo.PhoneNo
	ord.ShippingInfo.PostalCode = order.ShippingInfo.PostalCode
	ord.ShippingInfo.Country = order.ShippingInfo.Country
	ord.ItemPrice = int(itemPrice)
	ord.ShippingPrice = order.ShippingPrice
	ord.TaxPrice = order.TaxPrice
	ord.TotalPrice = int(totalPrice)
	ord.PaymentInfo.ID = order.PaymentInfo.ID
	ord.PaymentInfo.Status = order.PaymentInfo.Status
	ord.UserID = user.ID
	ord.PaidAt = time.Now()
	ord.OrderStatus = "Processing"
	ord.DeliveredAt = time.Time{}

	ord, err = h.ordersUC.CreateOrder(*ord)
	if err != nil {
		_ = utils.BadRequest(w, r, err)
		h.logger.Errorf("error creating order: %v", err)
		return
	}

	jr := models.OrderResponse{
		Success: true,
		Order:   *ord,
	}

	_ = utils.WriteJSON(w, http.StatusOK, jr)
}

// GetSingleOrder gets an order by id   =>  /api/v1/orders/:id
func (h *OrderHandlers) GetSingleOrder(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	parsedId, err := uuid.Parse(id)
	if err != nil {
		_ = utils.BadRequest(w, r, err)
		h.logger.Errorf("error parsing id: %v", err)
		return
	}

	order, err := h.ordersUC.GetSingleOrder(parsedId)
	if err != nil {
		_ = utils.BadRequest(w, r, err)
		h.logger.Errorf("error getting order: %v", err)
		return
	}

	jr := models.OrderResponse{
		Success: true,
		Order:   *order,
	}

	_ = utils.WriteJSON(w, http.StatusOK, jr)
}

// GetUserOrders gets logged-in user orders   =>   /api/v1/orders/me
func (h *OrderHandlers) GetUserOrders(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(UserContextKey).(*models.User)
	if !ok {
		_ = utils.BadRequest(w, r, errors.New("user is not logged in"))
		h.logger.Error("error getting user from context")
		return
	}

	ords, err := h.ordersUC.GetUserOrders(user.ID)
	if err != nil {
		_ = utils.BadRequest(w, r, err)
		h.logger.Errorf("error getting user orders: %v", err)
		return
	}

	jr := struct {
		Success bool            `json:"success"`
		Orders  []*models.Order `json:"orders"`
	}{
		Success: true,
		Orders:  ords,
	}

	_ = utils.WriteJSON(w, http.StatusOK, jr)
}

// GetAllOrders get all orders - ADMIN  =>   /api/v1/orders/admin/orders/
func (h *OrderHandlers) GetAllOrders(w http.ResponseWriter, r *http.Request) {
	ords, err := h.ordersUC.GetAllOrders()
	if err != nil {
		_ = utils.BadRequest(w, r, err)
		h.logger.Errorf("error getting orders: %v", err)
		return
	}

	var totalAmount = 0

	for _, ord := range ords {
		totalAmount += ord.TotalPrice
	}

	jr := struct {
		Success     bool            `json:"success"`
		TotalAmount int             `json:"totalAmount"`
		Orders      []*models.Order `json:"orders"`
	}{
		Success:     true,
		TotalAmount: totalAmount,
		Orders:      ords,
	}

	_ = utils.WriteJSON(w, http.StatusOK, jr)
}

// UpdateOrder process order - ADMIN  =>   /api/v1/orders/admin/order/:id
func (h *OrderHandlers) UpdateOrder(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	parsedId, err := uuid.Parse(id)
	if err != nil {
		_ = utils.BadRequest(w, r, err)
		h.logger.Errorf("error parsing id: %v", err)
		return
	}

	err = r.ParseMultipartForm(100000)
	if err != nil {
		_ = utils.BadRequest(w, r, err)
		h.logger.Errorf("error parsing form: %v", err)
		return
	}

	status := r.Form.Get("status")

	v := validator.New()

	v.Check(status != "", "status", "status field is empty")

	if !v.Valid() {
		_ = utils.BadRequest(w, r, errors.New("forms must be filled"))
		h.logger.Errorf("Form validation error")
		return
	}

	order, err := h.ordersUC.GetSingleOrder(parsedId)
	if err != nil {
		_ = utils.BadRequest(w, r, err)
		h.logger.Errorf("error fetching order: %v", err)
		return
	}

	if order.OrderStatus == "Delivered" {
		_ = utils.BadRequest(w, r, errors.New("you have already delivered this order"))
		h.logger.Infof("you have already delivered this order")
		return
	}

	for _, item := range order.OrderItems {
		err := h.ordersUC.UpdateStock(item.ProductID, item.Quantity)
		if err != nil {
			_ = utils.BadRequest(w, r, err)
			h.logger.Errorf("error updating stock: %v", err)
			return
		}
	}

	order.OrderStatus = status
	if status == "Delivered" {
		order.DeliveredAt = time.Now()
	} else {
		order.DeliveredAt = time.Time{}
	}

	err = h.ordersUC.UpdateOrder(*order)
	if err != nil {
		_ = utils.BadRequest(w, r, err)
		h.logger.Errorf("error updating order: %v", err)
		return
	}

	jsonRes := struct {
		Success bool `json:"success"`
	}{
		Success: true,
	}

	_ = utils.WriteJSON(w, http.StatusOK, jsonRes)
}

// DeleteOrder deletes order   =>   /api/v1/orders/admin/order/:id
func (h *OrderHandlers) DeleteOrder(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	parsedId, err := uuid.Parse(id)
	if err != nil {
		_ = utils.BadRequest(w, r, err)
		h.logger.Errorf("error parsing id: %v", err)
		return
	}

	err = h.ordersUC.DeleteOrder(parsedId)
	if err != nil {
		_ = utils.BadRequest(w, r, err)
		h.logger.Errorf("error deleting the order: %v", err)
		return
	}

	jsonRes := struct {
		Success bool `json:"success"`
	}{
		Success: true,
	}

	_ = utils.WriteJSON(w, http.StatusOK, jsonRes)
}
