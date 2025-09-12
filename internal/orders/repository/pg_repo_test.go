package repository_test

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/jofosuware/go/shopit/internal/models"
	"github.com/jofosuware/go/shopit/internal/orders/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInsertOrder(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Updated query includes delivered_at and a 9th argument.
	query := `insert into orders \(item_price, tax_price, shipping_price, total_price, order_status, paid_at, delivered_at, user_id, created_at\) values \(\$1, \$2, \$3, \$4, \$5, \$6, \$7, \$8, \$9\) returning order_id, item_price, tax_price, shipping_price, total_price, order_status, paid_at, delivered_at, user_id, created_at`

	order := models.Order{
		ItemPrice:     100,
		TaxPrice:      10,
		ShippingPrice: 20,
		TotalPrice:    130,
		OrderStatus:   "pending",
		PaidAt:        time.Now(),
		DeliveredAt:   time.Time{}, // freshly inserted order's DeliveredAt is empty
		UserID:        uuid.New(),
	}

	t.Run("Order inserted successfully", func(t *testing.T) {
		// For created_at we allow any argument.
		row := sqlmock.NewRows([]string{
			"order_id", "item_price", "tax_price", "shipping_price", "total_price", "order_status", "paid_at", "delivered_at", "user_id", "created_at",
		}).AddRow(uuid.New(), order.ItemPrice, order.TaxPrice, order.ShippingPrice, order.TotalPrice, order.OrderStatus, order.PaidAt, order.DeliveredAt, order.UserID, time.Now())

		mock.ExpectQuery(query).WithArgs(
			order.ItemPrice,
			order.TaxPrice,
			order.ShippingPrice,
			order.TotalPrice,
			order.OrderStatus,
			order.PaidAt,
			order.DeliveredAt,
			order.UserID,
			sqlmock.AnyArg(),
		).WillReturnRows(row)

		repo := repository.NewOrdersRepository(db)

		result, err := repo.InsertOrder(order)
		require.NoError(t, err)

		assert.NotNil(t, result)
		assert.Equal(t, order.ItemPrice, result.ItemPrice)
	})
}

func TestInsertItems(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	query := `insert into order_items \(name, price, quantity, image, product_id, order_id, created_at\)
				values \(\$1, \$2, \$3, \$4, \$5, \$6, \$7\) returning item_id, name, price, quantity, image,
				product_id, order_id, created_at
	`

	item := models.Item{
		Name:      "test_product",
		OrderID:   uuid.New(),
		ProductID: uuid.New(),
		Quantity:  2,
		Image:     "test_image.jpg",
		Price:     100,
	}

	t.Run("Items inserted successfully", func(t *testing.T) {
		row := sqlmock.NewRows([]string{"item_id", "name", "price", "quantity", "image", "product_id", "order_id", "created_at"}).
			AddRow(uuid.UUID{}, item.Name, item.Price, item.Quantity, item.Image, item.ProductID, item.OrderID, time.Now())

		mock.ExpectQuery(query).WithArgs(item.Name, item.Price, item.Quantity, item.Image, item.ProductID, item.OrderID, sqlmock.AnyArg()).WillReturnRows(row)

		repo := repository.NewOrdersRepository(db)

		i, err := repo.InsertItem(item)
		require.NoError(t, err)

		assert.NotNil(t, i)
		assert.Equal(t, item.OrderID, i.OrderID)
	})
}

func TestInsertPayment(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Updated query: now includes payment_id as first argument and four arguments in total.
	query := `insert into payments \(payment_id, status, order_id, created_at\) values \(\$1, \$2, \$3, \$4\) returning\s+payment_id, status, order_id, created_at`

	payment := models.Payment{
		ID:        "",
		Status:    "paid",
		OrderID:   uuid.New(),
		CreatedAt: time.Now(),
	}

	t.Run("Payment inserted successfully", func(t *testing.T) {
		row := sqlmock.NewRows([]string{"payment_id", "status", "order_id", "created_at"}).
			AddRow(payment.ID, payment.Status, payment.OrderID, time.Now())

		mock.ExpectQuery(query).WithArgs(payment.ID, payment.Status, payment.OrderID, sqlmock.AnyArg()).WillReturnRows(row)

		repo := repository.NewOrdersRepository(db)

		p, err := repo.InsertPayment(payment)
		require.NoError(t, err)

		assert.NotNil(t, p)
		assert.Equal(t, payment.OrderID, p.OrderID)
	})
}

func TestInsertShipping(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	query := `insert into shippings \(address, city, phone, postal, country, order_id, created_at\) values \(\$1, \$2, \$3, \$4, \$5, \$6, \$7\) returning
				shipping_id, address, city, phone, postal, country, order_id, created_at
	`

	shipping := models.Shipping{
		ID:         uuid.New(),
		Address:    "test_address",
		City:       "test_city",
		PhoneNo:    "test_phone_no",
		PostalCode: "test_postal_code",
		Country:    "test_country",
		OrderID:    uuid.New(),
		CreatedAt:  time.Now(),
	}

	t.Run("Shipping inserted successfully", func(t *testing.T) {
		row := sqlmock.NewRows([]string{"shipping_id", "address", "city", "phone", "postal", "country", "order_id", "created_at"}).
			AddRow(shipping.ID, shipping.Address, shipping.City, shipping.PhoneNo, shipping.PostalCode, shipping.Country, shipping.OrderID, shipping.CreatedAt)

		mock.ExpectQuery(query).WithArgs(shipping.Address, shipping.City, shipping.PhoneNo, shipping.PostalCode, shipping.Country, shipping.OrderID, sqlmock.AnyArg()).WillReturnRows(row)

		repo := repository.NewOrdersRepository(db)

		s, err := repo.InsertShipping(shipping)
		require.NoError(t, err)

		assert.NotNil(t, s)
		assert.Equal(t, shipping.OrderID, s.OrderID)
	})
}

func TestFetchOrderById(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	query := `select \* from orders where order_id = \$1`

	order := models.Order{
		OrderID:       uuid.New(),
		ItemPrice:     100,
		TaxPrice:      10,
		ShippingPrice: 20,
		TotalPrice:    130,
		OrderStatus:   "pending",
		PaidAt:        time.Now(),
		DeliveredAt:   time.Now(),
		UserID:        uuid.New(),
		CreatedAt:     time.Now(),
	}

	t.Run("Order fetched successfully", func(t *testing.T) {
		row := sqlmock.NewRows([]string{"order_id", "item_price", "tax_price", "shipping_price", "total_price", "order_status", "paid_at", "delivered_at", "user_id", "created_at"}).
			AddRow(order.OrderID, order.ItemPrice, order.TaxPrice, order.ShippingPrice, order.TotalPrice, order.OrderStatus, order.PaidAt, order.DeliveredAt, order.UserID, order.CreatedAt)

		mock.ExpectQuery(query).WithArgs(order.OrderID).WillReturnRows(row)

		repo := repository.NewOrdersRepository(db)

		o, err := repo.FetchOrderById(order.OrderID)
		require.NoError(t, err)

		assert.NotNil(t, o)
		assert.Equal(t, order.OrderID, o.OrderID)
	})
}

func TestFetchOrdersById(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// The query used in FetchOrdersById, matching the column order of Scan()
	query := `select order_id, item_price, tax_price, shipping_price, total_price, order_status, paid_at, delivered_at, user_id, created_at from orders where user_id = \$1`

	// Create a sample expected order.
	expOrder := models.Order{
		OrderID:       uuid.New(),
		ItemPrice:     100,
		TaxPrice:      10,
		ShippingPrice: 20,
		TotalPrice:    130,
		OrderStatus:   "pending",
		PaidAt:        time.Now(),
		DeliveredAt:   time.Now(),
		UserID:        uuid.New(),
		CreatedAt:     time.Now(),
	}

	t.Run("Orders fetched successfully", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{
			"order_id", "item_price", "tax_price", "shipping_price", "total_price", "order_status", "paid_at", "delivered_at", "user_id", "created_at",
		}).AddRow(
			expOrder.OrderID,
			expOrder.ItemPrice,
			expOrder.TaxPrice,
			expOrder.ShippingPrice,
			expOrder.TotalPrice,
			expOrder.OrderStatus,
			expOrder.PaidAt,
			expOrder.DeliveredAt,
			expOrder.UserID,
			expOrder.CreatedAt,
		)

		mock.ExpectQuery(query).WithArgs(expOrder.UserID).WillReturnRows(rows)

		repo := repository.NewOrdersRepository(db)
		orders, err := repo.FetchOrdersById(expOrder.UserID)
		require.NoError(t, err)
		require.NotNil(t, orders)
		assert.Len(t, orders, 1)
		assert.Equal(t, expOrder.OrderID, orders[0].OrderID)
	})
}

func TestFetchItemsById(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Updated query: selecting specific columns in the defined order.
	query := `select item_id, name, price, quantity, image, product_id, order_id, created_at from order_items where order_id = \$1`

	item := models.Item{
		ItemID:    uuid.New(),
		Name:      "test_name",
		Price:     100,
		Quantity:  3,
		Image:     "test_image",
		ProductID: uuid.New(),
		OrderID:   uuid.New(),
		CreatedAt: time.Now(),
	}

	t.Run("Items fetched successfully", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{
			"item_id", "name", "price", "quantity", "image", "product_id", "order_id", "created_at",
		}).AddRow(
			item.ItemID,
			item.Name,
			item.Price,
			item.Quantity,
			item.Image,
			item.ProductID,
			item.OrderID,
			item.CreatedAt,
		)

		mock.ExpectQuery(query).WithArgs(item.OrderID).WillReturnRows(rows)

		repo := repository.NewOrdersRepository(db)
		items, err := repo.FetchItemsById(item.OrderID)
		require.NoError(t, err)

		assert.NotNil(t, items)
		assert.Equal(t, item.OrderID, items[0].OrderID)
	})
}

func TestFetchPaymentById(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	query := `select \* from payments where order_id = \$1`

	payment := models.Payment{
		ID:        "unique_id",
		Status:    "paid",
		OrderID:   uuid.New(),
		CreatedAt: time.Now(),
	}

	t.Run("Payment fetched successfully", func(t *testing.T) {
		row := sqlmock.NewRows([]string{"id", "status", "order_id", "created_at"}).
			AddRow(payment.ID, payment.Status, payment.OrderID, payment.CreatedAt)

		mock.ExpectQuery(query).WithArgs(payment.OrderID).WillReturnRows(row)

		repo := repository.NewOrdersRepository(db)

		p, err := repo.FetchPaymentById(payment.OrderID)
		require.NoError(t, err)

		assert.NotNil(t, p)
		assert.Equal(t, payment.OrderID, p.OrderID)
	})
}

func TestFetchShippingById(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	query := `select \* from shippings where order_id = \$1`

	shipping := models.Shipping{
		ID:         uuid.New(),
		Address:    "test_address",
		City:       "test_city",
		PhoneNo:    "test_phone",
		PostalCode: "test_postal",
		Country:    "test_country",
		OrderID:    uuid.New(),
		CreatedAt:  time.Now(),
	}

	t.Run("Shipping fetched successfully", func(t *testing.T) {
		row := sqlmock.NewRows([]string{"id", "address", "city", "phone", "postal", "country", "order_id", "created_at"}).
			AddRow(shipping.ID, shipping.Address, shipping.City, shipping.PhoneNo, shipping.PostalCode, shipping.Country, shipping.OrderID, shipping.CreatedAt)

		mock.ExpectQuery(query).WithArgs(shipping.OrderID).WillReturnRows(row)

		repo := repository.NewOrdersRepository(db)

		s, err := repo.FetchShippingById(shipping.OrderID)
		require.NoError(t, err)

		assert.NotNil(t, s)
		assert.Equal(t, shipping.OrderID, s.OrderID)
	})
}

func TestDeleteOrderById(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	query := `delete from orders where order_id = \$1`

	orderId := uuid.New()

	t.Run("Order deleted successfully", func(t *testing.T) {
		mock.ExpectExec(query).WithArgs(orderId).WillReturnResult(sqlmock.NewResult(1, 1))

		repo := repository.NewOrdersRepository(db)

		err := repo.DeleteOrderById(orderId)
		require.NoError(t, err)
	})
}

func TestFetchAllOrders(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Updated query: selecting specific columns in the defined order.
	query := `select order_id, user_id, paid_at, item_price, tax_price, shipping_price, total_price, order_status, delivered_at, created_at from orders`

	// Create a sample expected order.
	ords := []*models.Order{
		{
			OrderID:       uuid.New(),
			UserID:        uuid.New(),
			PaidAt:        time.Now(),
			ItemPrice:     1,
			TaxPrice:      1,
			ShippingPrice: 1,
			TotalPrice:    5,
			OrderStatus:   "Processing",
			DeliveredAt:   time.Now(),
			CreatedAt:     time.Now(),
		},
	}

	t.Run("All orders successfully fetched", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{
			"order_id", "user_id", "paid_at", "item_price", "tax_price", "shipping_price", "total_price", "order_status", "delivered_at", "created_at",
		}).AddRow(
			ords[0].OrderID,
			ords[0].UserID,
			ords[0].PaidAt,
			ords[0].ItemPrice,
			ords[0].TaxPrice,
			ords[0].ShippingPrice,
			ords[0].TotalPrice,
			ords[0].OrderStatus,
			ords[0].DeliveredAt,
			ords[0].CreatedAt,
		)

		mock.ExpectQuery(query).WithArgs().WillReturnRows(rows)

		repo := repository.NewOrdersRepository(db)
		orders, err := repo.FetchAllOrders()
		require.NoError(t, err)

		assert.NotNil(t, orders)
		assert.Len(t, orders, 1)
		assert.Equal(t, ords[0].OrderID, orders[0].OrderID)
	})
}

func TestFetchAllItems(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Updated query: selecting specific columns in the defined order.
	query := `select item_id, name, price, quantity, image, product_id, order_id, created_at from order_items`

	item := models.Item{
		ItemID: uuid.New(),
		// Additional fields can be set as needed.
	}

	t.Run("Items are successfully fetched", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{
			"item_id", "name", "price", "quantity", "image", "product_id", "order_id", "created_at",
		}).AddRow(item.ItemID, item.Name, item.Price, item.Quantity, item.Image, item.ProductID, item.OrderID, item.CreatedAt)

		mock.ExpectQuery(query).WillReturnRows(rows)

		repo := repository.NewOrdersRepository(db)
		items, err := repo.FetchAllItems()
		require.NoError(t, err)

		assert.NotNil(t, items)
		assert.Equal(t, item.ItemID, items[0].ItemID)
	})
}

func TestFetchAllPayment(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Updated query: selecting specific columns from payments.
	query := `select payment_id, status, order_id, created_at from payments`

	payment := models.Payment{
		ID:        "test_id",
		Status:    "paid",
		OrderID:   uuid.New(),
		CreatedAt: time.Now(),
	}

	t.Run("Payments successfully fetched", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"payment_id", "status", "order_id", "created_at"}).
			AddRow(payment.ID, payment.Status, payment.OrderID, payment.CreatedAt)

		mock.ExpectQuery(query).WillReturnRows(rows)
		repo := repository.NewOrdersRepository(db)

		payments, err := repo.FetchAllPayment()
		require.NoError(t, err)

		assert.NotNil(t, payments)
		assert.Equal(t, payment.ID, payments[0].ID)
	})
}

func TestFetchAllShipping(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Updated query: selecting specific columns from shippings.
	query := `select shipping_id, address, city, phone, postal, country, order_id, created_at from shippings`

	// Create a sample shipping record.
	s := models.Shipping{
		ID:         uuid.New(),
		Address:    "Test Address",
		City:       "Test City",
		PhoneNo:    "12345678",
		PostalCode: "12345",
		Country:    "Testland",
		OrderID:    uuid.New(),
		CreatedAt:  time.Now(),
	}

	t.Run("Shippings successfully fetched", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{
			"shipping_id", "address", "city", "phone", "postal", "country", "order_id", "created_at",
		}).AddRow(
			s.ID,
			s.Address,
			s.City,
			s.PhoneNo,
			s.PostalCode,
			s.Country,
			s.OrderID,
			s.CreatedAt,
		)

		mock.ExpectQuery(query).WillReturnRows(rows)
		repo := repository.NewOrdersRepository(db)

		shipping, err := repo.FetchAllShipping()
		require.NoError(t, err)

		assert.NotNil(t, shipping)
		assert.Len(t, shipping, 1)
		assert.Equal(t, s.ID, shipping[0].ID)
	})
}

func TestUpdateStock(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	query := `update products set stock = stock - \$1 where product_id = \$2`

	productId := uuid.New()
	quantity := 5

	t.Run("Stock is successfully updated", func(t *testing.T) {
		mock.ExpectExec(query).WithArgs(quantity, productId).WillReturnResult(sqlmock.NewResult(1, 1))

		repo := repository.NewOrdersRepository(db)

		err := repo.UpdateStock(productId, quantity)
		require.NoError(t, err)
	})
}
