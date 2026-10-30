package usecase

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jofosuware/go/shopit/internal/models"
	"github.com/jofosuware/go/shopit/internal/orders"
	"github.com/jofosuware/go/shopit/pkg/card"
	"github.com/jofosuware/go/shopit/pkg/observability"
	"github.com/stripe/stripe-go/v72"
)

// OrderUC provides order-related use cases.
type OrderUC struct {
	repo orders.Repo
	card card.Carder
}

// NewOrderUC returns a new OrderUC.
func NewOrderUC(repo orders.Repo, payment card.Carder) *OrderUC {
	return &OrderUC{repo: repo, card: payment}
}

// CreatePaymentIntent creates a user-bound intent for the current catalog quote.
func (o *OrderUC) CreatePaymentIntent(ctx context.Context, userID uuid.UUID, items []*models.Item) (string, string, error) {
	ctx, span := observability.StartSpan(ctx, "payment.create_intent")
	defer span.Finish()
	if userID == uuid.Nil || len(items) == 0 || len(items) > orders.MaxOrderItems {
		return "", "", orders.ErrInvalidOrderRequest
	}
	if o.card == nil {
		return "", "", errors.New("payment provider is not configured")
	}
	for _, item := range items {
		if item == nil || item.ProductID == uuid.Nil || item.Quantity <= 0 {
			return "", "", orders.ErrInvalidOrderRequest
		}
	}
	_, amountDollars, err := o.repo.QuoteItems(ctx, items)
	if err != nil {
		return "", "", fmt.Errorf("quoting payment items: %w", err)
	}
	if amountDollars <= 0 || int64(amountDollars) > math.MaxInt64/100 {
		return "", "", orders.ErrInvalidOrderRequest
	}
	intent, _, err := o.card.CreatePaymentIntent(ctx, "usd", int64(amountDollars)*100, userID.String())
	if err != nil {
		return "", "", fmt.Errorf("creating payment intent: %w", err)
	}
	if intent == nil || intent.ID == "" || intent.ClientSecret == "" {
		return "", "", errors.New("payment provider returned an empty intent")
	}
	return intent.ID, intent.ClientSecret, nil
}

// CreateOrder verifies payment and persists an order using current product data.
func (o *OrderUC) CreateOrder(ctx context.Context, order models.Order) (*models.Order, error) {
	ctx, span := observability.StartSpan(ctx, "orders.create")
	defer span.Finish()
	if len(order.OrderItems) == 0 || len(order.OrderItems) > orders.MaxOrderItems ||
		order.UserID == uuid.Nil || order.PaymentInfo.ID == "" {
		return nil, orders.ErrInvalidOrderRequest
	}
	if o.card == nil {
		return nil, errors.New("payment provider is not configured")
	}
	for _, item := range order.OrderItems {
		if item == nil || item.ProductID == uuid.Nil || item.Quantity <= 0 {
			return nil, orders.ErrInvalidOrderRequest
		}
	}

	intent, err := o.card.RetrievePaymentIntent(ctx, order.PaymentInfo.ID)
	if err != nil {
		return nil, fmt.Errorf("retrieving payment intent: %w", err)
	}
	if intent == nil || intent.Status != stripe.PaymentIntentStatusSucceeded ||
		intent.Currency != "usd" ||
		intent.Metadata["user_id"] != order.UserID.String() ||
		intent.Metadata["purpose"] != "order_checkout" {
		return nil, orders.ErrInvalidPayment
	}
	used, err := o.repo.PaymentIntentUsed(ctx, order.PaymentInfo.ID)
	if err != nil {
		return nil, fmt.Errorf("checking payment intent reuse: %w", err)
	}
	if used {
		return nil, orders.ErrPaymentAlreadyUsed
	}
	refundPending, err := o.repo.PaymentIntentRefundPending(ctx, order.PaymentInfo.ID)
	if err != nil {
		return nil, fmt.Errorf("checking pending payment refund: %w", err)
	}
	if refundPending {
		err := o.repo.RefundPaymentIntentIfUnused(ctx, order.PaymentInfo.ID, o.card.RefundPaymentIntent)
		if errors.Is(err, orders.ErrPaymentAlreadyUsed) || errors.Is(err, orders.ErrPaymentRefundFailed) {
			return nil, err
		}
		if err != nil {
			return nil, errors.Join(orders.ErrPaymentUseUnknown, fmt.Errorf("retrying pending payment refund: %w", err))
		}
		return nil, orders.ErrPaymentAlreadyUsed
	}

	quotedItems, itemPrice, err := o.repo.QuoteItems(ctx, order.OrderItems)
	if err != nil {
		if errors.Is(err, orders.ErrInsufficientStock) || errors.Is(err, orders.ErrProductNotFound) {
			return nil, o.refundForCheckoutConflict(ctx, order.PaymentInfo.ID, err)
		}
		return nil, fmt.Errorf("quoting order items: %w", err)
	}
	if itemPrice <= 0 || int64(itemPrice) > math.MaxInt64/100 {
		return nil, orders.ErrInvalidOrderRequest
	}
	expectedAmount := int64(itemPrice) * 100
	if intent.Amount != expectedAmount {
		return nil, o.refundForCheckoutConflict(ctx, order.PaymentInfo.ID, orders.ErrOrderPriceChanged)
	}

	order.OrderItems = quotedItems
	order.ItemPrice = itemPrice
	order.TaxPrice = 0
	order.ShippingPrice = 0
	order.TotalPrice = itemPrice
	order.OrderStatus = "Processing"
	order.PaidAt = time.Now().UTC()
	order.PaymentInfo.Status = string(stripe.PaymentIntentStatusSucceeded)

	created, err := o.repo.CreateOrder(ctx, order)
	if err != nil {
		if errors.Is(err, orders.ErrInsufficientStock) || errors.Is(err, orders.ErrOrderPriceChanged) ||
			errors.Is(err, orders.ErrProductNotFound) {
			return nil, o.refundForCheckoutConflict(ctx, order.PaymentInfo.ID, err)
		}
		return nil, fmt.Errorf("creating order: %w", err)
	}
	return created, nil
}

func (o *OrderUC) refundForCheckoutConflict(ctx context.Context, paymentID string, cause error) error {
	ctx, span := observability.StartSpan(ctx, "orders.refund_checkout_conflict")
	defer span.Finish()
	err := o.repo.RefundPaymentIntentIfUnused(ctx, paymentID, o.card.RefundPaymentIntent)
	if errors.Is(err, orders.ErrPaymentAlreadyUsed) {
		return err
	}
	if errors.Is(err, orders.ErrPaymentRefundFailed) {
		return errors.Join(err, cause)
	}
	if err != nil {
		return errors.Join(orders.ErrPaymentUseUnknown, cause, fmt.Errorf("safely refunding payment: %w", err))
	}
	return fmt.Errorf("creating order: %w", cause)
}

// GetSingleOrder returns an order by ID with its related records.
func (o *OrderUC) GetSingleOrder(ctx context.Context, orderID uuid.UUID) (*models.Order, error) {
	ctx, span := observability.StartSpan(ctx, "orders.get")
	defer span.Finish()
	found, _, _, err := o.repo.FetchOrdersWithDetails(ctx, &orderID, nil, 1, 1)
	if err != nil {
		return nil, fmt.Errorf("fetching order: %w", err)
	}
	if len(found) == 0 {
		return nil, orders.ErrOrderNotFound
	}
	return found[0], nil
}

// GetUserOrders returns all orders for a specific user.
func (o *OrderUC) GetUserOrders(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]*models.Order, int, int64, error) {
	ctx, span := observability.StartSpan(ctx, "orders.list_user")
	defer span.Finish()
	ordersForUser, count, totalAmount, err := o.repo.FetchOrdersWithDetails(ctx, nil, &userID, page, pageSize)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("fetching user orders: %w", err)
	}
	return ordersForUser, count, totalAmount, nil
}

// GetAllOrders returns all orders and their related records.
func (o *OrderUC) GetAllOrders(ctx context.Context, page, pageSize int) ([]*models.Order, int, int64, error) {
	ctx, span := observability.StartSpan(ctx, "orders.list")
	defer span.Finish()
	ordersList, count, totalAmount, err := o.repo.FetchOrdersWithDetails(ctx, nil, nil, page, pageSize)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("fetching all orders: %w", err)
	}
	return ordersList, count, totalAmount, nil
}

// UpdateOrderStatus validates and atomically persists an order status change.
func (o *OrderUC) UpdateOrderStatus(ctx context.Context, orderID uuid.UUID, status string) error {
	ctx, span := observability.StartSpan(ctx, "orders.update_status")
	defer span.Finish()
	if status != "Processing" && status != "Delivered" {
		return orders.ErrInvalidOrderStatus
	}
	if err := o.repo.UpdateOrderStatus(ctx, orderID, status); err != nil {
		return fmt.Errorf("updating order status: %w", err)
	}
	return nil
}

// DeleteOrder deletes an order by ID.
func (o *OrderUC) DeleteOrder(ctx context.Context, orderID uuid.UUID) error {
	ctx, span := observability.StartSpan(ctx, "orders.delete")
	defer span.Finish()
	if err := o.repo.DeleteOrder(ctx, orderID); err != nil {
		return fmt.Errorf("deleting order: %w", err)
	}
	return nil
}
