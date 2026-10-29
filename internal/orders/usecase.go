package orders

import (
	"context"

	"github.com/google/uuid"
	"github.com/jofosuware/go/shopit/internal/models"
)

type OrderUC interface {
	// CreateOrder process and save orders, returns orders when successful and error when failed
	CreateOrder(ctx context.Context, order models.Order) (*models.Order, error)

	// CreatePaymentIntent quotes catalog prices before asking the provider for payment.
	CreatePaymentIntent(ctx context.Context, userID uuid.UUID, items []*models.Item) (string, string, error)

	// GetSingleOrder returns a single order by id, return error when failed
	GetSingleOrder(ctx context.Context, id uuid.UUID) (*models.Order, error)

	// GetUserOrders returns all orders for a user, return error when failed
	GetUserOrders(ctx context.Context, userId uuid.UUID, page, pageSize int) ([]*models.Order, int, int64, error)

	// GetAllOrders returns all orders and return an error when failed
	GetAllOrders(ctx context.Context, page, pageSize int) ([]*models.Order, int, int64, error)

	// UpdateStock updates the product's quantity, returns an error on failure
	UpdateOrderStatus(ctx context.Context, orderID uuid.UUID, status string) error

	// DeleteOrder deletes an order, returns an error on failure
	DeleteOrder(ctx context.Context, orderId uuid.UUID) error
}
