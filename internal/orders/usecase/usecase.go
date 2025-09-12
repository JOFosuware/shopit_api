package usecase

import (
	"github.com/google/uuid"
	"github.com/jofosuware/go/shopit/internal/models"
	"github.com/jofosuware/go/shopit/internal/orders"
)

// OrderUC is the type struct of Order UseCase
type OrderUC struct {
	repo orders.Repo
}

// NewOrderUC is the constructor for OrderUC
func NewOrderUC(repo orders.Repo) *OrderUC {
	return &OrderUC{
		repo: repo,
	}
}

// CreateOrder process and save orders, returns orders when successful and error when failed
func (o *OrderUC) CreateOrder(ord models.Order) (*models.Order, error) {
	order, err := o.repo.InsertOrder(ord)
	if err != nil {
		return nil, err
	}

	// Update the ShippingInfo's order id
	ord.ShippingInfo.OrderID = order.OrderID

	shipping, err := o.repo.InsertShipping(ord.ShippingInfo)
	if err != nil {
		err = o.repo.DeleteOrderById(order.OrderID)
		if err != nil {
			return nil, err
		}
		return nil, err
	}

	// Update the OrderItems order id
	ord.OrderItems[0].OrderID = order.OrderID

	item, err := o.repo.InsertItem(*ord.OrderItems[0])
	if err != nil {
		err = o.repo.DeleteOrderById(order.OrderID)
		if err != nil {
			return nil, err
		}
		return nil, err
	}

	// Update the PaymentInfo's order id
	ord.PaymentInfo.OrderID = order.OrderID

	payment, err := o.repo.InsertPayment(ord.PaymentInfo)
	if err != nil {
		err = o.repo.DeleteOrderById(order.OrderID)
		if err != nil {
			return nil, err
		}
		return nil, err
	}

	order.ShippingInfo = *shipping
	order.OrderItems = append(order.OrderItems, item)
	order.PaymentInfo = *payment

	return order, nil
}

// GetSingleOrder returns a single order by orderId, return error when failed
func (o *OrderUC) GetSingleOrder(orderId uuid.UUID) (*models.Order, error) {
	order, err := o.repo.FetchOrderById(orderId)
	if err != nil {
		return nil, err
	}

	shippings, err := o.repo.FetchShippingById(orderId)
	if err != nil {
		return nil, err
	}

	items, err := o.repo.FetchItemsById(orderId)
	if err != nil {
		return nil, err
	}

	payment, err := o.repo.FetchPaymentById(orderId)
	if err != nil {
		return nil, err
	}

	order.ShippingInfo = *shippings
	order.OrderItems = items
	order.PaymentInfo = *payment

	return order, nil
}

// GetUserOrders returns all orders for a user, return error when failed
func (o *OrderUC) GetUserOrders(userId uuid.UUID) ([]*models.Order, error) {
	ords, err := o.repo.FetchOrdersById(userId)
	if err != nil {
		return nil, err
	}

	for i, ord := range ords {
		shippings, err := o.repo.FetchShippingById(ord.OrderID)
		if err != nil {
			return nil, err
		}
		ords[i].ShippingInfo = *shippings
	}

	for i, ord := range ords {
		items, err := o.repo.FetchItemsById(ord.OrderID)
		if err != nil {
			return nil, err
		}

		ords[i].OrderItems = items
	}

	for i, ord := range ords {
		payment, err := o.repo.FetchPaymentById(ord.OrderID)
		if err != nil {
			return nil, err
		}

		ords[i].PaymentInfo = *payment
	}

	return ords, nil
}

// GetAllOrders returns all orders and return an error when failed
func (o *OrderUC) GetAllOrders() ([]*models.Order, error) {
	ords, err := o.repo.FetchAllOrders()
	if err != nil {
		return nil, err
	}

	shippings, err := o.repo.FetchAllShipping()
	if err != nil {
		return nil, err
	}

	for i, shipping := range shippings {
		ords[i].ShippingInfo = *shipping
	}

	items, err := o.repo.FetchAllItems()
	if err != nil {
		return nil, err
	}

	for i, item := range items {
		ords[i].OrderItems = append(ords[i].OrderItems, item)
	}

	payments, err := o.repo.FetchAllPayment()
	if err != nil {
		return nil, err
	}

	for i, payment := range payments {
		ords[i].PaymentInfo = *payment
	}

	return ords, nil
}

// UpdateOrder updates an order, returns an error on failure
func (o *OrderUC) UpdateOrder(order models.Order) error {
	err := o.repo.UpdateOrder(order.OrderID, order)
	if err != nil {
		return err
	}

	return nil
}

// UpdateStock updates the product's quantity, returns an error on failure
func (o *OrderUC) UpdateStock(productId uuid.UUID, quantity int) error {
	err := o.repo.UpdateStock(productId, quantity)
	if err != nil {
		return nil
	}

	return nil
}

// DeleteOrder deletes an order, returns an error on failure
func (o *OrderUC) DeleteOrder(orderId uuid.UUID) error {
	err := o.repo.DeleteOrderById(orderId)
	if err != nil {
		return err
	}

	return nil
}
