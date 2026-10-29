package card

import (
	"context"
	"errors"
	"sync"

	"github.com/stripe/stripe-go/v72"
	"github.com/stripe/stripe-go/v72/paymentintent"
	"github.com/stripe/stripe-go/v72/refund"
)

var stripeKeyOnce sync.Once

func configureStripeKey(secret string) {
	stripeKeyOnce.Do(func() {
		stripe.Key = secret
	})
}

// Carder is the interface to card type
type Carder interface {
	// CreatePaymentIntent attempts to get a payment intent object from Stripe
	CreatePaymentIntent(ctx context.Context, currency string, amountCents int64, userID string) (*stripe.PaymentIntent, string, error)
	RetrievePaymentIntent(ctx context.Context, id string) (*stripe.PaymentIntent, error)
	RefundPaymentIntent(ctx context.Context, id string) error
}

// Card holds the information needed by this package
type Card struct {
	Secret   string
	Key      string
	Currency string
}

// CreatePaymentIntent attempts to get a payment intent object from Stripe
func (c *Card) CreatePaymentIntent(ctx context.Context, currency string, amountCents int64, userID string) (*stripe.PaymentIntent, string, error) {
	if amountCents <= 0 || userID == "" || currency != "usd" || c.Secret == "" {
		return nil, "", errors.New("payment amount, currency, user ID, and provider credentials must be valid")
	}
	configureStripeKey(c.Secret)

	params := &stripe.PaymentIntentParams{
		Amount:   stripe.Int64(amountCents),
		Currency: stripe.String(currency),
	}
	params.Context = ctx
	params.AddMetadata("integration_check", "accept_a_payment")
	params.AddMetadata("user_id", userID)
	params.AddMetadata("purpose", "order_checkout")

	pi, err := paymentintent.New(params)
	if err != nil {
		msg := ""
		if stripeErr, ok := err.(*stripe.Error); ok {
			msg = cardErrorMessage(stripeErr.Code)
		}
		return nil, msg, err
	}
	return pi, "", nil
}

// RetrievePaymentIntent returns the provider's current state for an intent.
func (c *Card) RetrievePaymentIntent(ctx context.Context, id string) (*stripe.PaymentIntent, error) {
	if id == "" || c.Secret == "" {
		return nil, errors.New("payment intent ID and provider credentials are required")
	}
	configureStripeKey(c.Secret)
	params := &stripe.PaymentIntentParams{}
	params.Context = ctx
	return paymentintent.Get(id, params)
}

// RefundPaymentIntent refunds a succeeded payment intent.
func (c *Card) RefundPaymentIntent(ctx context.Context, id string) error {
	if id == "" || c.Secret == "" {
		return errors.New("payment intent ID and provider credentials are required")
	}
	configureStripeKey(c.Secret)
	params := &stripe.RefundParams{PaymentIntent: stripe.String(id)}
	params.Context = ctx
	params.SetIdempotencyKey("order-refund-" + id)
	_, err := refund.New(params)
	return err
}

// cardErrorMessage returns human-readable versions of card error messages
func cardErrorMessage(code stripe.ErrorCode) string {
	var msg = ""
	switch code {
	case stripe.ErrorCodeCardDeclined:
		msg = "Your card was declined"
	case stripe.ErrorCodeExpiredCard:
		msg = "Your card is expired"
	case stripe.ErrorCodeIncorrectCVC:
		msg = "Incorrect CVC code"
	case stripe.ErrorCodeIncorrectZip:
		msg = "Incorrect zip/postal code"
	case stripe.ErrorCodeAmountTooLarge:
		msg = "The amount is too large to charge to your card"
	case stripe.ErrorCodeAmountTooSmall:
		msg = "The amount is too small to charge to your card"
	case stripe.ErrorCodeBalanceInsufficient:
		msg = "Insufficient balance"
	case stripe.ErrorCodePostalCodeInvalid:
		msg = "Your postal code is invalid"
	default:
		msg = "Your card was declined"
	}
	return msg
}
