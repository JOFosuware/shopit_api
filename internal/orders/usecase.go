package orders

import (
	"github.com/google/uuid"
	"github.com/jofosuware/go/shopit/internal/models"
)

type OrderUC interface {
	// CreateOrder process and save orders, returns orders when successful and error when failed
	CreateOrder(order models.Order) (*models.Order, error)

	// GetSingleOrder returns a single order by id, return error when failed
	GetSingleOrder(id uuid.UUID) (*models.Order, error)

	// GetUserOrders returns all orders for a user, return error when failed
	GetUserOrders(userId uuid.UUID) ([]*models.Order, error)

	// GetAllOrders returns all orders and return an error when failed
	GetAllOrders() ([]*models.Order, error)

	// UpdateStock updates the product's quantity, returns an error on failure
	UpdateStock(productId uuid.UUID, quantity int) error

	// UpdateOrder updates an order, returns an error on failure
	UpdateOrder(order models.Order) error

	// DeleteOrder deletes an order, returns an error on failure
	DeleteOrder(orderId uuid.UUID) error
}
