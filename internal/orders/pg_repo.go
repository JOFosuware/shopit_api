package orders

import (
	"github.com/google/uuid"
	"github.com/jofosuware/go/shopit/internal/models"
)

type Repo interface {
	// InsertOrder inserts an order into the database, returns the order and error on failure
	InsertOrder(order models.Order) (*models.Order, error)

	// InsertItem inserts order item into the database, returns the order items and error on failure
	InsertItem(i models.Item) (*models.Item, error)

	// InsertPayment inserts an order payment into the database, returns the order payment and error on failure
	InsertPayment(p models.Payment) (*models.Payment, error)

	// InsertShipping inserts an order shipment into the database, returns the order shipment and error on failure
	InsertShipping(s models.Shipping) (*models.Shipping, error)

	// FetchOrderById fetches an order by orderId, returns the order and error on failure
	FetchOrderById(orderId uuid.UUID) (*models.Order, error)

	// FetchOrdersById fetches orders by userID, returns the orders and error on failure
	FetchOrdersById(userID uuid.UUID) ([]*models.Order, error)

	// FetchAllOrders fetches all orders, returns the orders and error on failure
	FetchAllOrders() ([]*models.Order, error)

	// FetchItemsById fetches items by orderId, returns the items and an error on failure
	FetchItemsById(orderId uuid.UUID) ([]*models.Item, error)

	// FetchAllItems fetches all items, returns items and an error on failure
	FetchAllItems() ([]*models.Item, error)

	// FetchPaymentById fetches payment by orderId, returns the payment and an error on failure
	FetchPaymentById(orderId uuid.UUID) (*models.Payment, error)

	// FetchAllPayment fetches all payment, return payments and an error on failure
	FetchAllPayment() ([]*models.Payment, error)

	// FetchShippingById fetches shipping by orderId, returns the shipping and an error on failure
	FetchShippingById(orderId uuid.UUID) (*models.Shipping, error)

	// FetchAllShipping fetches all shipping, return shipping and an error on failure
	FetchAllShipping() ([]*models.Shipping, error)

	// DeleteOrderById deletes order by orderId and returns an error if failed
	DeleteOrderById(orderId uuid.UUID) error

	// UpdateOrder updates an order in the database, returns an error on failure
	UpdateOrder(orderId uuid.UUID, ord models.Order) error

	// UpdateStock updates the product's stock, returns an error on failure
	UpdateStock(productId uuid.UUID, quantity int) error
}
