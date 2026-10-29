package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jofosuware/go/shopit/internal/models"
	"github.com/jofosuware/go/shopit/internal/orders"
	"github.com/jofosuware/go/shopit/internal/orders/mocks"
	"github.com/jofosuware/go/shopit/internal/orders/usecase"
	mockCard "github.com/jofosuware/go/shopit/pkg/card/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v72"
)

func TestCreatePaymentIntentUsesCatalogQuoteInUSDMinorUnits(t *testing.T) {
	repo := mocks.NewRepo(t)
	carder := mockCard.NewCarder(t)
	uc := usecase.NewOrderUC(repo, carder)
	ctx := context.Background()
	userID, productID := uuid.New(), uuid.New()
	items := []*models.Item{{ProductID: productID, Quantity: 3}}
	repo.On("QuoteItems", ctx, items).
		Return([]*models.Item{{ProductID: productID, Name: "Item", Price: 4, Quantity: 3}}, 12, nil).Once()
	carder.On("CreatePaymentIntent", ctx, "usd", int64(1200), userID.String()).
		Return(&stripe.PaymentIntent{ID: "pi_quoted", ClientSecret: "secret"}, "", nil).Once()

	paymentID, clientSecret, err := uc.CreatePaymentIntent(ctx, userID, items)
	require.NoError(t, err)
	require.Equal(t, "pi_quoted", paymentID)
	require.Equal(t, "secret", clientSecret)
}

func TestCreateOrderDelegatesAtomicCreation(t *testing.T) {
	repo := mocks.NewRepo(t)
	carder := mockCard.NewCarder(t)
	uc := usecase.NewOrderUC(repo, carder)
	ctx := context.Background()
	order := models.Order{
		UserID:       uuid.New(),
		OrderItems:   []*models.Item{{ProductID: uuid.New(), Quantity: 2}},
		ShippingInfo: models.Shipping{Address: "1 Main St"},
		PaymentInfo:  models.Payment{ID: "payment-1"},
	}
	created := &models.Order{OrderID: uuid.New(), UserID: order.UserID}
	quoted := []*models.Item{{ProductID: order.OrderItems[0].ProductID, Price: 5, Quantity: 2, Name: "Item"}}
	repo.On("QuoteItems", ctx, order.OrderItems).Return(quoted, 10, nil).Once()
	carder.On("RetrievePaymentIntent", ctx, "payment-1").Return(&stripe.PaymentIntent{
		Status: stripe.PaymentIntentStatusSucceeded, Currency: "usd", Amount: 1000,
		Metadata: map[string]string{"user_id": order.UserID.String(), "purpose": "order_checkout"},
	}, nil).Once()
	repo.On("PaymentIntentUsed", ctx, "payment-1").Return(false, nil).Once()
	repo.On("PaymentIntentRefundPending", ctx, "payment-1").Return(false, nil).Once()
	repo.On("CreateOrder", ctx, mock.MatchedBy(func(got models.Order) bool {
		return got.UserID == order.UserID && got.ItemPrice == 10 && got.TotalPrice == 10 &&
			got.TaxPrice == 0 && got.ShippingPrice == 0 && got.OrderStatus == "Processing" &&
			got.PaymentInfo.Status == "succeeded"
	})).Return(created, nil).Once()

	got, err := uc.CreateOrder(ctx, order)
	require.NoError(t, err)
	require.Same(t, created, got)
}

func TestCreateOrderReturnsRepositoryFailure(t *testing.T) {
	repo := mocks.NewRepo(t)
	carder := mockCard.NewCarder(t)
	uc := usecase.NewOrderUC(repo, carder)
	ctx := context.Background()
	order := models.Order{UserID: uuid.New(), PaymentInfo: models.Payment{ID: "pi_1"}, OrderItems: []*models.Item{{ProductID: uuid.New(), Quantity: 1}}}
	createErr := errors.New("transaction failed")
	repo.On("QuoteItems", ctx, order.OrderItems).Return([]*models.Item{{ProductID: order.OrderItems[0].ProductID, Price: 2, Quantity: 1}}, 2, nil).Once()
	carder.On("RetrievePaymentIntent", ctx, "pi_1").Return(&stripe.PaymentIntent{
		Status: stripe.PaymentIntentStatusSucceeded, Currency: "usd", Amount: 200,
		Metadata: map[string]string{"user_id": order.UserID.String(), "purpose": "order_checkout"},
	}, nil).Once()
	repo.On("PaymentIntentUsed", ctx, "pi_1").Return(false, nil).Once()
	repo.On("PaymentIntentRefundPending", ctx, "pi_1").Return(false, nil).Once()
	repo.On("CreateOrder", ctx, mock.Anything).Return((*models.Order)(nil), createErr).Once()

	got, err := uc.CreateOrder(ctx, order)
	require.Nil(t, got)
	require.ErrorIs(t, err, createErr)
}

func TestCreateOrderRefundsCheckoutIntentWhenPriceChanges(t *testing.T) {
	repo := mocks.NewRepo(t)
	carder := mockCard.NewCarder(t)
	uc := usecase.NewOrderUC(repo, carder)
	ctx := context.Background()
	userID, productID := uuid.New(), uuid.New()
	order := models.Order{
		UserID: userID, PaymentInfo: models.Payment{ID: "pi_wrong_amount"},
		OrderItems: []*models.Item{{ProductID: productID, Quantity: 1}},
	}
	carder.On("RetrievePaymentIntent", ctx, "pi_wrong_amount").Return(&stripe.PaymentIntent{
		Status: stripe.PaymentIntentStatusSucceeded, Currency: "usd", Amount: 500,
		Metadata: map[string]string{"user_id": userID.String(), "purpose": "order_checkout"},
	}, nil).Once()
	repo.On("PaymentIntentUsed", ctx, "pi_wrong_amount").Return(false, nil).Once()
	repo.On("PaymentIntentRefundPending", ctx, "pi_wrong_amount").Return(false, nil).Once()
	repo.On("QuoteItems", ctx, order.OrderItems).
		Return([]*models.Item{{ProductID: productID, Price: 10, Quantity: 1}}, 10, nil).Once()
	carder.On("RefundPaymentIntent", ctx, "pi_wrong_amount").Return(nil).Once()
	repo.On("RefundPaymentIntentIfUnused", ctx, "pi_wrong_amount", mock.Anything).
		Run(func(args mock.Arguments) {
			refund := args.Get(2).(func(context.Context, string) error)
			require.NoError(t, refund(ctx, "pi_wrong_amount"))
		}).Return(nil).Once()

	created, err := uc.CreateOrder(ctx, order)
	require.Nil(t, created)
	require.ErrorIs(t, err, orders.ErrOrderPriceChanged)
	repo.AssertNotCalled(t, "CreateOrder", mock.Anything, mock.Anything)
}

func TestCreateOrderRefundsKnownInventoryConflict(t *testing.T) {
	repo := mocks.NewRepo(t)
	carder := mockCard.NewCarder(t)
	uc := usecase.NewOrderUC(repo, carder)
	ctx := context.Background()
	userID, productID := uuid.New(), uuid.New()
	order := models.Order{
		UserID: userID, PaymentInfo: models.Payment{ID: "pi_out_of_stock"},
		OrderItems: []*models.Item{{ProductID: productID, Quantity: 1}},
	}
	repo.On("QuoteItems", ctx, order.OrderItems).
		Return([]*models.Item{{ProductID: productID, Price: 10, Quantity: 1}}, 10, nil).Once()
	carder.On("RetrievePaymentIntent", ctx, "pi_out_of_stock").Return(&stripe.PaymentIntent{
		Status: stripe.PaymentIntentStatusSucceeded, Currency: "usd", Amount: 1000,
		Metadata: map[string]string{"user_id": userID.String(), "purpose": "order_checkout"},
	}, nil).Once()
	repo.On("PaymentIntentUsed", ctx, "pi_out_of_stock").Return(false, nil).Once()
	repo.On("PaymentIntentRefundPending", ctx, "pi_out_of_stock").Return(false, nil).Once()
	repo.On("CreateOrder", ctx, mock.Anything).
		Return((*models.Order)(nil), orders.ErrInsufficientStock).Once()
	carder.On("RefundPaymentIntent", ctx, "pi_out_of_stock").Return(nil).Once()
	repo.On("RefundPaymentIntentIfUnused", ctx, "pi_out_of_stock", mock.Anything).
		Run(func(args mock.Arguments) {
			refund := args.Get(2).(func(context.Context, string) error)
			require.NoError(t, refund(ctx, "pi_out_of_stock"))
		}).Return(nil).Once()

	created, err := uc.CreateOrder(ctx, order)
	require.Nil(t, created)
	require.ErrorIs(t, err, orders.ErrInsufficientStock)
}

func TestCreateOrderDoesNotRefundIntentAlreadyUsed(t *testing.T) {
	repo := mocks.NewRepo(t)
	carder := mockCard.NewCarder(t)
	uc := usecase.NewOrderUC(repo, carder)
	ctx := context.Background()
	userID, productID := uuid.New(), uuid.New()
	order := models.Order{
		UserID: userID, PaymentInfo: models.Payment{ID: "pi_reused"},
		OrderItems: []*models.Item{{ProductID: productID, Quantity: 1}},
	}
	carder.On("RetrievePaymentIntent", ctx, "pi_reused").Return(&stripe.PaymentIntent{
		Status: stripe.PaymentIntentStatusSucceeded, Currency: "usd", Amount: 1000,
		Metadata: map[string]string{"user_id": userID.String(), "purpose": "order_checkout"},
	}, nil).Once()
	repo.On("PaymentIntentUsed", ctx, "pi_reused").Return(true, nil).Once()

	created, err := uc.CreateOrder(ctx, order)
	require.Nil(t, created)
	require.ErrorIs(t, err, orders.ErrPaymentAlreadyUsed)
	repo.AssertNotCalled(t, "QuoteItems", mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "CreateOrder", mock.Anything, mock.Anything)
	carder.AssertNotCalled(t, "RefundPaymentIntent", mock.Anything, mock.Anything)
}

func TestCreateOrderRetriesPendingRefundInsteadOfCreatingOrder(t *testing.T) {
	repo := mocks.NewRepo(t)
	carder := mockCard.NewCarder(t)
	uc := usecase.NewOrderUC(repo, carder)
	ctx := context.Background()
	userID, productID := uuid.New(), uuid.New()
	order := models.Order{
		UserID: userID, PaymentInfo: models.Payment{ID: "pi_pending_refund"},
		OrderItems: []*models.Item{{ProductID: productID, Quantity: 1}},
	}
	carder.On("RetrievePaymentIntent", ctx, "pi_pending_refund").Return(&stripe.PaymentIntent{
		Status: stripe.PaymentIntentStatusSucceeded, Currency: "usd", Amount: 1000,
		Metadata: map[string]string{"user_id": userID.String(), "purpose": "order_checkout"},
	}, nil).Once()
	repo.On("PaymentIntentUsed", ctx, "pi_pending_refund").Return(false, nil).Once()
	repo.On("PaymentIntentRefundPending", ctx, "pi_pending_refund").Return(true, nil).Once()
	carder.On("RefundPaymentIntent", ctx, "pi_pending_refund").Return(nil).Once()
	repo.On("RefundPaymentIntentIfUnused", ctx, "pi_pending_refund", mock.Anything).
		Run(func(args mock.Arguments) {
			refund := args.Get(2).(func(context.Context, string) error)
			require.NoError(t, refund(ctx, "pi_pending_refund"))
		}).Return(nil).Once()

	created, err := uc.CreateOrder(ctx, order)
	require.Nil(t, created)
	require.ErrorIs(t, err, orders.ErrPaymentAlreadyUsed)
	repo.AssertNotCalled(t, "QuoteItems", mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "CreateOrder", mock.Anything, mock.Anything)
}

func TestGetAllOrdersLoadsRelatedRecordsFromRepository(t *testing.T) {
	repo := mocks.NewRepo(t)
	uc := usecase.NewOrderUC(repo, nil)
	ctx := context.Background()
	orderID := uuid.New()
	order := &models.Order{OrderID: orderID, ShippingInfo: models.Shipping{Address: "A"}}
	repo.On("FetchOrdersWithDetails", ctx, (*uuid.UUID)(nil), (*uuid.UUID)(nil), 1, 20).
		Return([]*models.Order{order}, 1, int64(0), nil).Once()

	got, _, _, err := uc.GetAllOrders(ctx, 1, 20)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, orderID, got[0].OrderID)
	require.Equal(t, "A", got[0].ShippingInfo.Address)
}

func TestGetSingleOrderReturnsNotFoundWhenDetailsQueryHasNoRows(t *testing.T) {
	repo := mocks.NewRepo(t)
	uc := usecase.NewOrderUC(repo, nil)
	ctx := context.Background()
	orderID := uuid.New()
	repo.On("FetchOrdersWithDetails", ctx, mock.MatchedBy(func(id *uuid.UUID) bool {
		return id != nil && *id == orderID
	}), (*uuid.UUID)(nil), 1, 1).Return([]*models.Order{}, 0, int64(0), nil).Once()

	got, err := uc.GetSingleOrder(ctx, orderID)
	require.Nil(t, got)
	require.ErrorIs(t, err, orders.ErrOrderNotFound)
}

func TestUpdateOrderStatusRejectsUnsupportedStatus(t *testing.T) {
	repo := mocks.NewRepo(t)
	uc := usecase.NewOrderUC(repo, nil)

	err := uc.UpdateOrderStatus(context.Background(), uuid.New(), "Cancelled")
	require.ErrorIs(t, err, orders.ErrInvalidOrderStatus)
	repo.AssertNotCalled(t, "UpdateOrderStatus", mock.Anything, mock.Anything, mock.Anything)
}
