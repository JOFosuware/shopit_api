// Package repository provides database access for orders.
package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"

	"github.com/jofosuware/go/shopit/internal/models"
)

// OrdersRepository handles order-related persistence operations.
type OrdersRepository struct {
	// DB is the database connection.
	DB *sql.DB
}

// NewOrdersRepository returns a new OrdersRepository.
func NewOrdersRepository(db *sql.DB) *OrdersRepository {
	return &OrdersRepository{DB: db}
}

// InsertOrder inserts an order into the database.
func (o *OrdersRepository) InsertOrder(order models.Order) (*models.Order, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `insert into orders (item_price, tax_price, shipping_price, total_price, order_status,
				paid_at, delivered_at, user_id, created_at) values ($1, $2, $3, $4, $5, $6, $7, $8, $9) returning 
				order_id, item_price, tax_price, shipping_price, total_price, order_status, paid_at, delivered_at,
				user_id, created_at`

	err := o.DB.QueryRowContext(ctx, query,
		order.ItemPrice,
		order.TaxPrice,
		order.ShippingPrice,
		order.TotalPrice,
		order.OrderStatus,
		order.PaidAt,
		order.DeliveredAt,
		order.UserID,
		time.Now(),
	).Scan(
		&order.OrderID,
		&order.ItemPrice,
		&order.TaxPrice,
		&order.ShippingPrice,
		&order.TotalPrice,
		&order.OrderStatus,
		&order.PaidAt,
		&order.DeliveredAt,
		&order.UserID,
		&order.CreatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &order, nil
}

// InsertItem inserts an order item into the database.
func (o *OrdersRepository) InsertItem(item models.Item) (*models.Item, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `insert into order_items (name, price, quantity, image, product_id, order_id, created_at)
				values ($1, $2, $3, $4, $5, $6, $7) returning item_id, name, price, quantity, image,
				product_id, order_id, created_at
	`
	err := o.DB.QueryRowContext(ctx, query,
		item.Name,
		item.Price,
		item.Quantity,
		item.Image,
		item.ProductID,
		item.OrderID,
		time.Now(),
	).Scan(
		&item.ItemID,
		&item.Name,
		&item.Price,
		&item.Quantity,
		&item.Image,
		&item.ProductID,
		&item.OrderID,
		&item.CreatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &item, nil
}

// InsertPayment inserts a payment record for an order.
func (o *OrdersRepository) InsertPayment(p models.Payment) (*models.Payment, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `insert into payments (payment_id, status, order_id, created_at) values ($1, $2, $3, $4) returning
				payment_id, status, order_id, created_at
	`
	err := o.DB.QueryRowContext(ctx, query,
		p.ID,
		p.Status,
		p.OrderID,
		time.Now(),
	).Scan(
		&p.ID,
		&p.Status,
		&p.OrderID,
		&p.CreatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &p, nil
}

// InsertShipping inserts shipping information for an order.
func (o *OrdersRepository) InsertShipping(shipping models.Shipping) (*models.Shipping, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `insert into shippings (address, city, phone, postal, country, order_id, created_at) values ($1, $2, $3, $4, $5, $6, $7) returning
				shipping_id, address, city, phone, postal, country, order_id, created_at
	`
	err := o.DB.QueryRowContext(ctx, query,
		shipping.Address,
		shipping.City,
		shipping.PhoneNo,
		shipping.PostalCode,
		shipping.Country,
		shipping.OrderID,
		time.Now(),
	).Scan(
		&shipping.ID,
		&shipping.Address,
		&shipping.City,
		&shipping.PhoneNo,
		&shipping.PostalCode,
		&shipping.Country,
		&shipping.OrderID,
		&shipping.CreatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &shipping, nil
}

// FetchOrderById fetches an order by its ID.
func (o *OrdersRepository) FetchOrderById(id uuid.UUID) (*models.Order, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `select * from orders where order_id = $1`
	var order models.Order
	err := o.DB.QueryRowContext(ctx, query, id).Scan(
		&order.OrderID,
		&order.ItemPrice,
		&order.TaxPrice,
		&order.ShippingPrice,
		&order.TotalPrice,
		&order.OrderStatus,
		&order.PaidAt,
		&order.DeliveredAt,
		&order.UserID,
		&order.CreatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &order, nil
}

// FetchOrdersById fetches orders for a specific user.
func (o *OrdersRepository) FetchOrdersById(userID uuid.UUID) ([]*models.Order, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `select order_id, item_price, tax_price, shipping_price, total_price, order_status, paid_at, delivered_at,
				user_id, created_at from orders where user_id = $1`

	rows, err := o.DB.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []*models.Order

	for rows.Next() {
		var order models.Order
		err := rows.Scan(
			&order.OrderID,
			&order.ItemPrice,
			&order.TaxPrice,
			&order.ShippingPrice,
			&order.TotalPrice,
			&order.OrderStatus,
			&order.PaidAt,
			&order.DeliveredAt,
			&order.UserID,
			&order.CreatedAt,
		)

		if err != nil {
			return nil, err
		}

		orders = append(orders, &order)
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}

	return orders, nil
}

// FetchItemsById fetches items for a given order ID.
func (o *OrdersRepository) FetchItemsById(orderId uuid.UUID) ([]*models.Item, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `select item_id, name, price, quantity, image, product_id, order_id, created_at from order_items where order_id = $1`

	rows, err := o.DB.QueryContext(ctx, query, orderId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*models.Item

	for rows.Next() {
		var item models.Item
		err := rows.Scan(
			&item.ItemID,
			&item.Name,
			&item.Price,
			&item.Quantity,
			&item.Image,
			&item.ProductID,
			&item.OrderID,
			&item.CreatedAt,
		)

		if err != nil {
			return nil, err
		}

		items = append(items, &item)
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}

	return items, nil
}

// FetchPaymentById fetches the payment record for an order.
func (o *OrdersRepository) FetchPaymentById(orderId uuid.UUID) (*models.Payment, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `select * from payments where order_id = $1`

	var payment models.Payment

	err := o.DB.QueryRowContext(ctx, query, orderId).Scan(
		&payment.ID,
		&payment.Status,
		&payment.OrderID,
		&payment.CreatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &payment, nil
}

// FetchShippingById fetches shipping information for an order.
func (o *OrdersRepository) FetchShippingById(orderId uuid.UUID) (*models.Shipping, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `select * from shippings where order_id = $1`

	var shipping models.Shipping

	err := o.DB.QueryRowContext(ctx, query, orderId).Scan(
		&shipping.ID,
		&shipping.Address,
		&shipping.City,
		&shipping.PhoneNo,
		&shipping.PostalCode,
		&shipping.Country,
		&shipping.OrderID,
		&shipping.CreatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &shipping, nil
}

// DeleteOrderById deletes an order by its ID.
func (o *OrdersRepository) DeleteOrderById(orderId uuid.UUID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `delete from orders where order_id = $1`
	_, err := o.DB.ExecContext(ctx, query, orderId)
	if err != nil {
		return err
	}

	return nil
}

// FetchAllOrders returns all orders.
func (o *OrdersRepository) FetchAllOrders() ([]*models.Order, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `select order_id, user_id, paid_at, item_price, tax_price, shipping_price, 
		total_price, order_status, delivered_at, created_at from orders`

	rows, err := o.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ords []*models.Order

	for rows.Next() {
		var ord models.Order

		err := rows.Scan(
			&ord.OrderID,
			&ord.UserID,
			&ord.PaidAt,
			&ord.ItemPrice,
			&ord.TaxPrice,
			&ord.ShippingPrice,
			&ord.TotalPrice,
			&ord.OrderStatus,
			&ord.DeliveredAt,
			&ord.CreatedAt,
		)

		if err != nil {
			return nil, err
		}

		ords = append(ords, &ord)

		if err := rows.Err(); err != nil {
			return nil, err
		}
	}

	return ords, nil
}

// FetchAllItems returns all order items.
func (o *OrdersRepository) FetchAllItems() ([]*models.Item, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `select item_id, name, price, quantity, image, product_id, order_id, created_at from order_items`

	rows, err := o.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}

	var items []*models.Item

	for rows.Next() {
		var item models.Item

		err = rows.Scan(
			&item.ItemID,
			&item.Name,
			&item.Price,
			&item.Quantity,
			&item.Image,
			&item.ProductID,
			&item.OrderID,
			&item.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		items = append(items, &item)

		if err := rows.Err(); err != nil {
			return nil, err
		}
	}

	return items, nil
}

// FetchAllPayment returns all payments.
func (o *OrdersRepository) FetchAllPayment() ([]*models.Payment, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `select payment_id, status, order_id, created_at from payments`

	rows, err := o.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}

	var payments []*models.Payment

	for rows.Next() {
		var payment models.Payment
		err := rows.Scan(
			&payment.ID,
			&payment.Status,
			&payment.OrderID,
			&payment.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		payments = append(payments, &payment)

		if err := rows.Err(); err != nil {
			return nil, err
		}
	}

	return payments, err
}

// FetchAllShipping returns all shipping records.
func (o *OrdersRepository) FetchAllShipping() ([]*models.Shipping, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `select shipping_id, address, city, phone, postal, country, order_id,
		created_at from shippings`

	rows, err := o.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}

	var shipping []*models.Shipping

	for rows.Next() {
		var s models.Shipping
		err = rows.Scan(
			&s.ID,
			&s.Address,
			&s.City,
			&s.PhoneNo,
			&s.PostalCode,
			&s.Country,
			&s.OrderID,
			&s.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		shipping = append(shipping, &s)

		if err := rows.Err(); err != nil {
			return nil, err
		}
	}

	return shipping, nil
}

// UpdateOrder updates an order's status and delivered time.
func (o *OrdersRepository) UpdateOrder(orderId uuid.UUID, ord models.Order) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `update orders set order_status = $1, delivered_at = $2 where order_id = $3`

	_, err := o.DB.ExecContext(ctx, query, ord.OrderStatus, ord.DeliveredAt, orderId)
	if err != nil {
		return err
	}

	return nil
}

// UpdateStock decrements a product's stock by the given quantity.
func (o *OrdersRepository) UpdateStock(productId uuid.UUID, quantity int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `update products set stock = stock - $1 where product_id = $2`

	_, err := o.DB.ExecContext(ctx, query, quantity, productId)
	if err != nil {
		return err
	}

	return nil
}
