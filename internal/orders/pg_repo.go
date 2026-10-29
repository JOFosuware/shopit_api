package orders

import (
	"context"

	"github.com/google/uuid"
	"github.com/jofosuware/go/shopit/internal/models"
)

type Repo interface {
	CreateOrder(ctx context.Context, order models.Order) (*models.Order, error)
	QuoteItems(ctx context.Context, items []*models.Item) ([]*models.Item, int, error)
	PaymentIntentUsed(ctx context.Context, paymentID string) (bool, error)
	PaymentIntentRefundPending(ctx context.Context, paymentID string) (bool, error)
	RefundPaymentIntentIfUnused(ctx context.Context, paymentID string, refund func(context.Context, string) error) error
	FetchOrdersWithDetails(ctx context.Context, orderID, userID *uuid.UUID, page, pageSize int) ([]*models.Order, int, int64, error)
	DeleteOrder(ctx context.Context, orderID uuid.UUID) error
	UpdateOrderStatus(ctx context.Context, orderID uuid.UUID, status string) error
}
