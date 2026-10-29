package repository_test

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/jofosuware/go/shopit/internal/models"
	"github.com/jofosuware/go/shopit/internal/orders"
	"github.com/jofosuware/go/shopit/internal/orders/repository"
	"github.com/stretchr/testify/require"
)

func TestCreateOrderPersistsAllRecordsAtomically(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := repository.NewOrdersRepository(db)

	orderID, userID := uuid.New(), uuid.New()
	productIDs := []uuid.UUID{
		uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		uuid.MustParse("00000000-0000-0000-0000-000000000002"),
	}
	createdAt := time.Now()
	order := models.Order{
		UserID:      userID,
		ItemPrice:   26,
		TotalPrice:  26,
		OrderStatus: "Processing",
		PaidAt:      createdAt,
		OrderItems: []*models.Item{
			{ProductID: productIDs[0], Name: "First", Price: 12, Quantity: 1, Image: "first.jpg"},
			{ProductID: productIDs[1], Name: "Second", Price: 7, Quantity: 2, Image: "second.jpg"},
		},
		ShippingInfo: models.Shipping{Address: "1 Main St"},
		PaymentInfo:  models.Payment{ID: "payment-1", Status: "succeeded"},
	}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`select pg_advisory_xact_lock(hashtextextended($1, 0)) is null`)).
		WithArgs("payment-1").WillReturnRows(sqlmock.NewRows([]string{"locked"}).AddRow(false))
	mock.ExpectQuery(`(?s)select exists\(select 1 from payments where payment_id = \$1\).*exists\(select 1 from order_payment_refunds where payment_id = \$1\)`).
		WithArgs("payment-1").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectQuery(`(?s)select p\.name, p\.price, p\.stock.*where p\.product_id = \$1\s+for update`).
		WithArgs(productIDs[0]).
		WillReturnRows(sqlmock.NewRows([]string{"name", "price", "stock", "image"}).AddRow("First", 12, 4, "first.jpg"))
	mock.ExpectExec(`update products set stock = stock - \$1`).
		WithArgs(1, productIDs[0]).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`(?s)select p\.name, p\.price, p\.stock.*where p\.product_id = \$1\s+for update`).
		WithArgs(productIDs[1]).
		WillReturnRows(sqlmock.NewRows([]string{"name", "price", "stock", "image"}).AddRow("Second", 7, 5, "second.jpg"))
	mock.ExpectExec(`update products set stock = stock - \$1`).
		WithArgs(2, productIDs[1]).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`(?s)insert into orders .*returning order_id`).
		WithArgs(26, float64(0), 0, 26, "Processing", createdAt, nil, userID).
		WillReturnRows(sqlmock.NewRows([]string{
			"order_id", "item_price", "tax_price", "shipping_price", "total_price",
			"order_status", "paid_at", "delivered_at", "user_id", "created_at",
		}).AddRow(orderID, 26, 0, 0, 26, "Processing", createdAt, nil, userID, createdAt))
	mock.ExpectQuery(`(?s)insert into shippings .*returning shipping_id`).
		WithArgs("1 Main St", "", "", "", "", orderID).
		WillReturnRows(sqlmock.NewRows([]string{
			"shipping_id", "address", "city", "phone", "postal", "country", "order_id", "created_at",
		}).AddRow(uuid.New(), "1 Main St", "", "", "", "", orderID, createdAt))
	for i, item := range order.OrderItems {
		mock.ExpectQuery(`(?s)insert into order_items .*returning item_id`).
			WithArgs(item.Name, item.Price, item.Quantity, item.Image, item.ProductID, orderID).
			WillReturnRows(sqlmock.NewRows([]string{
				"item_id", "name", "price", "quantity", "image", "product_id", "order_id", "created_at",
			}).AddRow(uuid.New(), item.Name, item.Price, item.Quantity, item.Image, productIDs[i], orderID, createdAt))
	}
	mock.ExpectQuery(`(?s)insert into payments .*returning payment_id`).
		WithArgs("payment-1", "succeeded", orderID).
		WillReturnRows(sqlmock.NewRows([]string{"payment_id", "status", "order_id", "created_at"}).
			AddRow("payment-1", "succeeded", orderID, createdAt))
	mock.ExpectCommit()

	created, err := repo.CreateOrder(context.Background(), order)
	require.NoError(t, err)
	require.Equal(t, orderID, created.OrderID)
	require.Len(t, created.OrderItems, 2)
	require.Equal(t, orderID, created.OrderItems[1].OrderID)
	require.Equal(t, "payment-1", created.PaymentInfo.ID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestQuoteItemsUsesCatalogDataAndAggregatesDuplicateProducts(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := repository.NewOrdersRepository(db)
	productID := uuid.New()
	items := []*models.Item{
		{ProductID: productID, Quantity: 1, Name: "client name", Price: 1},
		{ProductID: productID, Quantity: 2, Name: "tampered", Price: 500},
	}
	mock.ExpectQuery(`(?s)select p\.name, p\.price, p\.stock.*where p\.product_id = \$1`).
		WithArgs(productID).
		WillReturnRows(sqlmock.NewRows([]string{"name", "price", "stock", "image"}).
			AddRow("Trusted name", 4, 5, "trusted.jpg"))

	quoted, total, err := repo.QuoteItems(context.Background(), items)
	require.NoError(t, err)
	require.Equal(t, 12, total)
	require.Equal(t, []*models.Item{{
		ProductID: productID, Name: "Trusted name", Price: 4, Quantity: 3, Image: "trusted.jpg",
	}}, quoted)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateOrderRejectsCatalogPriceChangeBeforeReservingStock(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := repository.NewOrdersRepository(db)
	productID := uuid.New()
	order := models.Order{
		ItemPrice: 8, PaymentInfo: models.Payment{ID: "pi_price_change"},
		OrderItems: []*models.Item{{ProductID: productID, Price: 8, Quantity: 1}},
	}
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`select pg_advisory_xact_lock(hashtextextended($1, 0)) is null`)).
		WithArgs("pi_price_change").WillReturnRows(sqlmock.NewRows([]string{"locked"}).AddRow(false))
	mock.ExpectQuery(`(?s)select exists\(select 1 from payments where payment_id = \$1\).*exists\(select 1 from order_payment_refunds where payment_id = \$1\)`).
		WithArgs("pi_price_change").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectQuery(`(?s)select p\.name, p\.price, p\.stock.*where p\.product_id = \$1\s+for update`).
		WithArgs(productID).
		WillReturnRows(sqlmock.NewRows([]string{"name", "price", "stock", "image"}).
			AddRow("Product", 9, 3, "product.jpg"))
	mock.ExpectRollback()

	_, err = repo.CreateOrder(context.Background(), order)
	require.ErrorIs(t, err, orders.ErrOrderPriceChanged)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateOrderRollsBackWhenRelatedRecordFails(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := repository.NewOrdersRepository(db)
	userID := uuid.New()

	productID := uuid.New()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`select pg_advisory_xact_lock(hashtextextended($1, 0)) is null`)).
		WithArgs("pi-rollback").WillReturnRows(sqlmock.NewRows([]string{"locked"}).AddRow(false))
	mock.ExpectQuery(`(?s)select exists\(select 1 from payments where payment_id = \$1\).*exists\(select 1 from order_payment_refunds where payment_id = \$1\)`).
		WithArgs("pi-rollback").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectQuery(`(?s)select p\.name, p\.price, p\.stock.*where p\.product_id = \$1\s+for update`).
		WithArgs(productID).
		WillReturnRows(sqlmock.NewRows([]string{"name", "price", "stock", "image"}).AddRow("First", 5, 2, "first.jpg"))
	mock.ExpectExec(`update products set stock = stock - \$1`).
		WithArgs(1, productID).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`(?s)insert into orders .*returning order_id`).
		WithArgs(5, float64(0), 0, 5, "Processing", sqlmock.AnyArg(), nil, userID).
		WillReturnRows(sqlmock.NewRows([]string{
			"order_id", "item_price", "tax_price", "shipping_price", "total_price",
			"order_status", "paid_at", "delivered_at", "user_id", "created_at",
		}).AddRow(uuid.New(), 5, 0, 0, 5, "Processing", nil, nil, userID, time.Now()))
	mock.ExpectQuery(`(?s)insert into shippings .*returning shipping_id`).
		WithArgs("", "", "", "", "", sqlmock.AnyArg()).
		WillReturnError(errors.New("shipping unavailable"))
	mock.ExpectRollback()

	_, err = repo.CreateOrder(context.Background(), models.Order{
		UserID:      userID,
		ItemPrice:   5,
		OrderStatus: "Processing",
		PaidAt:      time.Now(),
		OrderItems:  []*models.Item{{ProductID: productID, Price: 5, Quantity: 1}},
		PaymentInfo: models.Payment{ID: "pi-rollback", Status: "succeeded"},
	})
	require.ErrorContains(t, err, "inserting order shipping")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateOrderDoesNotReusePaymentIntent(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := repository.NewOrdersRepository(db)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`select pg_advisory_xact_lock(hashtextextended($1, 0)) is null`)).
		WithArgs("pi-used").WillReturnRows(sqlmock.NewRows([]string{"locked"}).AddRow(false))
	mock.ExpectQuery(`(?s)select exists\(select 1 from payments where payment_id = \$1\).*exists\(select 1 from order_payment_refunds where payment_id = \$1\)`).
		WithArgs("pi-used").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectRollback()

	_, err = repo.CreateOrder(context.Background(), models.Order{
		PaymentInfo: models.Payment{ID: "pi-used"},
		OrderItems:  []*models.Item{{ProductID: uuid.New(), Quantity: 1}},
	})
	require.ErrorIs(t, err, orders.ErrPaymentAlreadyUsed)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRefundPaymentIntentIfUnusedPersistsClaimBeforeProviderCall(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := repository.NewOrdersRepository(db)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`select pg_advisory_xact_lock(hashtextextended($1, 0)) is null`)).
		WithArgs("pi_refund").WillReturnRows(sqlmock.NewRows([]string{"locked"}).AddRow(false))
	mock.ExpectQuery(regexp.QuoteMeta(`select exists(select 1 from payments where payment_id = $1)`)).
		WithArgs("pi_refund").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectQuery(regexp.QuoteMeta(`select status from order_payment_refunds where payment_id = $1 for update`)).
		WithArgs("pi_refund").WillReturnError(sql.ErrNoRows)
	mock.ExpectExec(`(?s)insert into order_payment_refunds .*values \(\$1, 'pending', now\(\), now\(\)\)`).
		WithArgs("pi_refund").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectExec(`(?s)update order_payment_refunds .*status = 'refunded'.*where payment_id = \$1 and status = 'pending'`).
		WithArgs("pi_refund").WillReturnResult(sqlmock.NewResult(0, 1))

	refundCalled := false
	err = repo.RefundPaymentIntentIfUnused(context.Background(), "pi_refund", func(_ context.Context, id string) error {
		refundCalled = true
		require.Equal(t, "pi_refund", id)
		return nil
	})
	require.NoError(t, err)
	require.True(t, refundCalled)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRefundPaymentIntentIfUnusedNeverRefundsAnOrderedIntent(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := repository.NewOrdersRepository(db)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`select pg_advisory_xact_lock(hashtextextended($1, 0)) is null`)).
		WithArgs("pi_used").WillReturnRows(sqlmock.NewRows([]string{"locked"}).AddRow(false))
	mock.ExpectQuery(regexp.QuoteMeta(`select exists(select 1 from payments where payment_id = $1)`)).
		WithArgs("pi_used").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectRollback()

	err = repo.RefundPaymentIntentIfUnused(context.Background(), "pi_used", func(context.Context, string) error {
		t.Fatal("must not refund an intent already associated with an order")
		return nil
	})
	require.ErrorIs(t, err, orders.ErrPaymentAlreadyUsed)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateOrderStatusRejectsUnpaidOrder(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := repository.NewOrdersRepository(db)
	orderID := uuid.New()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`select order_status, paid_at from orders where order_id = $1 for update`)).
		WithArgs(orderID).
		WillReturnRows(sqlmock.NewRows([]string{"order_status", "paid_at"}).AddRow("Processing", nil))
	mock.ExpectRollback()

	err = repo.UpdateOrderStatus(context.Background(), orderID, "Delivered")
	require.ErrorIs(t, err, orders.ErrInvalidPayment)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateOrderStatusChangesFulfillmentWithoutChangingStock(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := repository.NewOrdersRepository(db)
	orderID := uuid.New()
	mock.ExpectBegin()
	paidAt := time.Now()
	mock.ExpectQuery(regexp.QuoteMeta(`select order_status, paid_at from orders where order_id = $1 for update`)).
		WithArgs(orderID).
		WillReturnRows(sqlmock.NewRows([]string{"order_status", "paid_at"}).AddRow("Processing", paidAt))
	mock.ExpectExec(`update orders set order_status = \$1, delivered_at = now\(\) where order_id = \$2`).
		WithArgs("Delivered", orderID).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	require.NoError(t, repo.UpdateOrderStatus(context.Background(), orderID, "Delivered"))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFetchOrdersWithDetailsGroupsItemsPerOrderID(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := repository.NewOrdersRepository(db)

	orderA, orderB := uuid.New(), uuid.New()
	userID := uuid.New()
	productA, productB := uuid.New(), uuid.New()
	createdAt := time.Now()
	rows := sqlmock.NewRows([]string{
		"order_id", "item_price", "tax_price", "shipping_price", "total_price", "order_status",
		"paid_at", "delivered_at", "user_id", "created_at",
		"shipping_id", "address", "city", "phone", "postal", "country", "shipping_order_id", "shipping_created_at",
		"item_id", "name", "price", "quantity", "image", "product_id", "item_order_id", "item_created_at",
		"payment_id", "status", "payment_order_id", "payment_created_at",
	})
	rows.AddRow(orderA, 10, 0, 0, 10, "Processing", nil, nil, userID, createdAt,
		uuid.New(), "A", "", "", "", "", orderA, createdAt,
		uuid.New(), "A1", 10, 1, "a.jpg", productA, orderA, createdAt,
		"payment-a", "pending", orderA, createdAt)
	rows.AddRow(orderA, 10, 0, 0, 10, "Processing", nil, nil, userID, createdAt,
		uuid.New(), "A", "", "", "", "", orderA, createdAt,
		uuid.New(), "A2", 0, 1, "a2.jpg", uuid.New(), orderA, createdAt,
		"payment-a", "pending", orderA, createdAt)
	rows.AddRow(orderB, 20, 0, 0, 20, "Processing", nil, nil, userID, createdAt,
		uuid.New(), "B", "", "", "", "", orderB, createdAt,
		uuid.New(), "B1", 20, 1, "b.jpg", productB, orderB, createdAt,
		"payment-b", "pending", orderB, createdAt)
	mock.ExpectQuery(`select count\(\*\), coalesce\(sum\(o\.total_price\), 0\) from orders o`).
		WillReturnRows(sqlmock.NewRows([]string{"count", "total"}).AddRow(2, 30))
	mock.ExpectQuery(`(?s)with paged_orders as .*from shippings.*join order_items.*from payments.*order by o\.created_at desc`).
		WithArgs(20, 0).
		WillReturnRows(rows)

	result, count, totalAmount, err := repo.FetchOrdersWithDetails(context.Background(), nil, nil, 1, 20)
	require.NoError(t, err)
	require.Equal(t, 2, count)
	require.Equal(t, int64(30), totalAmount)
	require.Len(t, result, 2)
	require.Equal(t, orderA, result[0].OrderID)
	require.Len(t, result[0].OrderItems, 2)
	require.Equal(t, "A", result[0].ShippingInfo.Address)
	require.Equal(t, "payment-a", result[0].PaymentInfo.ID)
	require.Equal(t, orderB, result[1].OrderID)
	require.Len(t, result[1].OrderItems, 1)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFetchOrdersWithDetailsRejectsInvalidPageWithoutQuerying(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := repository.NewOrdersRepository(db)

	result, count, total, err := repo.FetchOrdersWithDetails(context.Background(), nil, nil, 1, 101)
	require.Nil(t, result)
	require.Zero(t, count)
	require.Zero(t, total)
	require.ErrorIs(t, err, orders.ErrInvalidPagination)
	require.NoError(t, mock.ExpectationsWereMet())
}
