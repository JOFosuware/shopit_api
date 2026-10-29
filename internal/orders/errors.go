package orders

import "errors"

const MaxOrderItems = 100

var (
	ErrOrderNotFound         = errors.New("order not found")
	ErrInsufficientStock     = errors.New("insufficient product stock")
	ErrOrderAlreadyDelivered = errors.New("order is already delivered")
	ErrInvalidOrderStatus    = errors.New("unsupported order status")
	ErrInvalidOrderRequest   = errors.New("invalid order request")
	ErrInvalidPayment        = errors.New("payment is invalid or has not succeeded")
	ErrPaymentAlreadyUsed    = errors.New("payment intent is already associated with an order")
	ErrPaymentRefundFailed   = errors.New("payment refund failed")
	ErrPaymentUseUnknown     = errors.New("payment intent use status is unknown")
	ErrProductNotFound       = errors.New("product not found")
	ErrOrderPriceChanged     = errors.New("order price changed; retry checkout")
	ErrInvalidPagination     = errors.New("invalid pagination")
)
