package usecase_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jofosuware/go/shopit/internal/models"
	"github.com/jofosuware/go/shopit/internal/orders/mocks"
	"github.com/jofosuware/go/shopit/internal/orders/usecase"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCreateOrder(t *testing.T) {
	repo := mocks.NewRepo(t)
	o := usecase.NewOrderUC(repo)

	t.Run("Order is successfully created", func(t *testing.T) {
		order := &models.Order{
			OrderID:      uuid.New(),
			ShippingInfo: models.Shipping{},
			OrderItems: []*models.Item{
				{
					ItemID:    uuid.New(),
					Name:      "Test",
					ProductID: uuid.New(),
					OrderID:   uuid.New(),
					CreatedAt: time.Now(),
				},
			},
			PaymentInfo:   models.Payment{},
			ItemPrice:     0,
			TaxPrice:      0,
			ShippingPrice: 0,
			TotalPrice:   0,
			UserID: 	  uuid.New(),
			PaidAt: 	time.Now(),
			OrderStatus:   "Processing",
			DeliveredAt:   time.Time{},
		}

		 // Use matchers to allow the ShippingInfo to have an updated OrderID.
		repo.On("InsertOrder", *order).Return(order, nil)
		repo.
			On("InsertShipping", mock.MatchedBy(func(s models.Shipping) bool {
				// We expect the OrderID to have been populated (i.e. not the zero value).
				return s.OrderID != uuid.Nil
			})).
			Return(&models.Shipping{}, nil)
		repo.
			On("InsertItem", mock.AnythingOfType("models.Item")).
			Return(&models.Item{}, nil)
		repo.
			On("InsertPayment", mock.AnythingOfType("models.Payment")).
			Return(&models.Payment{}, nil)

		// Call CreateOrder which should also update fields such as PaidAt and OrderStatus.
		createdOrder, err := o.CreateOrder(*order)
		require.NoError(t, err)

		// Assertions to match the changes in the CreateOrder method.
		assert.NotNil(t, createdOrder)
		assert.Equal(t, "Test", createdOrder.OrderItems[0].Name)

		// New checks: Verify that OrderStatus is set to "Processing" and PaidAt is non-zero.
		assert.Equal(t, "Processing", createdOrder.OrderStatus)
		assert.False(t, createdOrder.PaidAt.IsZero(), "PaidAt timestamp should be set")
	})
}

func TestGetSingleOrder(t *testing.T) {
	repo := mocks.NewRepo(t)

	o := usecase.NewOrderUC(repo)

	t.Run("Order is successfully retrieved", func(t *testing.T) {
		id := uuid.New()

		repo.On("FetchOrderById", id).Return(&models.Order{UserID: id}, nil)
		repo.On("FetchShippingById", id).Return(&models.Shipping{}, nil)
		repo.On("FetchItemsById", id).Return([]*models.Item{}, nil)
		repo.On("FetchPaymentById", id).Return(&models.Payment{}, nil)

		order, err := o.GetSingleOrder(id)
		require.NoError(t, err)

		assert.NotNil(t, order)
		assert.Equal(t, order.UserID, id)
	})
}

func TestGetUserOrders(t *testing.T) {
	repo := mocks.NewRepo(t)

	o := usecase.NewOrderUC(repo)

	t.Run("Orders are successfully retrieved", func(t *testing.T) {
		userId := uuid.New()
		orderId := uuid.New()

		repo.On("FetchOrdersById", userId).Return([]*models.Order{{UserID: userId, OrderID: orderId}}, nil)
		repo.On("FetchShippingById", orderId).Return(&models.Shipping{}, nil)
		repo.On("FetchItemsById", orderId).Return([]*models.Item{}, nil)
		repo.On("FetchPaymentById", orderId).Return(&models.Payment{}, nil)

		orders, err := o.GetUserOrders(userId)
		require.NoError(t, err)

		assert.NotNil(t, orders)
		assert.Equal(t, orders[0].UserID, userId)
	})
}

func TestGetAllOrders(t *testing.T) {
	repo := mocks.NewRepo(t)

	o := usecase.NewOrderUC(repo)

	t.Run("All orders are successfully retrieved", func(t *testing.T) {

		repo.On("FetchAllOrders").Return([]*models.Order{}, nil)
		repo.On("FetchAllShipping").Return([]*models.Shipping{}, nil)
		repo.On("FetchAllItems").Return([]*models.Item{}, nil)
		repo.On("FetchAllPayment").Return([]*models.Payment{}, nil)

		orders, err := o.GetAllOrders()
		require.NoError(t, err)

		assert.NotNil(t, orders)
	})
}

func TestUpdateOrder(t *testing.T) {
	repo := mocks.NewRepo(t)

	o := usecase.NewOrderUC(repo)

	t.Run("Order is successfully updated", func(t *testing.T) {
		ord := models.Order{}

		repo.On("UpdateOrder", ord.OrderID, ord).Return(nil)

		err := o.UpdateOrder(ord)
		require.NoError(t, err)
	})
}

func TestUpdateStock(t *testing.T) {
	repo := mocks.NewRepo(t)

	o := usecase.NewOrderUC(repo)

	t.Run("Stock is successfully updated", func(t *testing.T) {
		ord := models.Order{
			OrderItems: []*models.Item{
				{
					ProductID: uuid.New(),
					Quantity:  5,
				},
			},
		}

		repo.On("UpdateStock", ord.OrderItems[0].ProductID, ord.OrderItems[0].Quantity).Return(nil)

		err := o.UpdateStock(ord.OrderItems[0].ProductID, ord.OrderItems[0].Quantity)
		require.NoError(t, err)
	})
}

func TestDeleteOrder(t *testing.T) {
	repo := mocks.NewRepo(t)

	o := usecase.NewOrderUC(repo)

	t.Run("Order is successfully deleted", func(t *testing.T) {
		id := uuid.New()

		repo.On("DeleteOrderById", id).Return(nil)

		err := o.DeleteOrder(id)
		require.NoError(t, err)
	})
}
