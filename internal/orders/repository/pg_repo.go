// Package repository provides database access for orders.
package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/jofosuware/go/shopit/internal/models"
	"github.com/jofosuware/go/shopit/internal/orders"
	"github.com/jofosuware/go/shopit/pkg/observability"
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

func aggregateQuantities(items []*models.Item) (map[uuid.UUID]int, []uuid.UUID, error) {
	quantities := make(map[uuid.UUID]int, len(items))
	for _, item := range items {
		if item == nil || item.ProductID == uuid.Nil || item.Quantity <= 0 ||
			quantities[item.ProductID] > math.MaxInt-item.Quantity {
			return nil, nil, orders.ErrInvalidOrderRequest
		}
		quantities[item.ProductID] += item.Quantity
	}
	productIDs := make([]uuid.UUID, 0, len(quantities))
	for productID := range quantities {
		productIDs = append(productIDs, productID)
	}
	sort.Slice(productIDs, func(i, j int) bool {
		return productIDs[i].String() < productIDs[j].String()
	})
	return quantities, productIDs, nil
}

func orderItemPrice(items []*models.Item, productID uuid.UUID) int {
	for _, item := range items {
		if item != nil && item.ProductID == productID {
			return item.Price
		}
	}
	return -1
}

// QuoteItems returns one trusted product snapshot per distinct product ID.
func (o *OrdersRepository) QuoteItems(parent context.Context, items []*models.Item) ([]*models.Item, int, error) {
	_, span := observability.StartSpan(parent, "db.orders.quote_items")
	defer span.Finish()
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()

	quantities, productIDs, err := aggregateQuantities(items)
	if err != nil {
		return nil, 0, err
	}

	quoted := make([]*models.Item, 0, len(productIDs))
	var total int64
	for _, productID := range productIDs {
		var name, image string
		var price, stock int
		err := o.DB.QueryRowContext(ctx, `
			select p.name, p.price, p.stock,
				coalesce((
					select i.url from images i
					where i.product_id = p.product_id
					order by i.created_at, i.public_id limit 1
				), '')
			from products p
			where p.product_id = $1
		`, productID).Scan(&name, &price, &stock, &image)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, 0, orders.ErrProductNotFound
		}
		if err != nil {
			return nil, 0, fmt.Errorf("loading product %s for quote: %w", productID, err)
		}
		if price < 0 || stock < quantities[productID] {
			return nil, 0, orders.ErrInsufficientStock
		}
		if int64(price) > math.MaxInt64/int64(quantities[productID]) {
			return nil, 0, orders.ErrInvalidOrderRequest
		}
		lineTotal := int64(price) * int64(quantities[productID])
		if total > int64(math.MaxInt)-lineTotal {
			return nil, 0, orders.ErrInvalidOrderRequest
		}
		total += lineTotal
		quoted = append(quoted, &models.Item{
			ProductID: productID,
			Name:      name,
			Price:     price,
			Quantity:  quantities[productID],
			Image:     image,
		})
	}
	return quoted, int(total), nil
}

// PaymentIntentUsed reports whether an intent is already attached to an order.
func (o *OrdersRepository) PaymentIntentUsed(parent context.Context, paymentID string) (bool, error) {
	_, span := observability.StartSpan(parent, "db.orders.payment_intent_used")
	defer span.Finish()
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()
	if paymentID == "" {
		return false, orders.ErrInvalidOrderRequest
	}
	var used bool
	if err := o.DB.QueryRowContext(ctx,
		`select exists(select 1 from payments where payment_id = $1)
			or exists(select 1 from order_payment_refunds where payment_id = $1 and status = 'refunded')`, paymentID,
	).Scan(&used); err != nil {
		return false, fmt.Errorf("checking payment intent reuse: %w", err)
	}
	return used, nil
}

// PaymentIntentRefundPending reports whether a previous refund attempt needs retry.
func (o *OrdersRepository) PaymentIntentRefundPending(parent context.Context, paymentID string) (bool, error) {
	_, span := observability.StartSpan(parent, "db.orders.payment_refund_pending")
	defer span.Finish()
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()
	if paymentID == "" {
		return false, orders.ErrInvalidOrderRequest
	}
	var pending bool
	if err := o.DB.QueryRowContext(ctx, `
		select exists(
			select 1 from order_payment_refunds
			where payment_id = $1 and status = 'pending'
		)
	`, paymentID).Scan(&pending); err != nil {
		return false, fmt.Errorf("checking pending payment refund: %w", err)
	}
	return pending, nil
}

// RefundPaymentIntentIfUnused records a durable refund claim before invoking the provider.
func (o *OrdersRepository) RefundPaymentIntentIfUnused(
	parent context.Context,
	paymentID string,
	refundPayment func(context.Context, string) error,
) error {
	_, span := observability.StartSpan(parent, "db.orders.refund_payment_intent")
	defer span.Finish()
	if paymentID == "" || refundPayment == nil {
		return orders.ErrInvalidOrderRequest
	}
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()

	tx, err := o.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning payment refund claim: %w", err)
	}
	defer tx.Rollback()

	var lockAcquired bool
	if err := tx.QueryRowContext(ctx,
		`select pg_advisory_xact_lock(hashtextextended($1, 0)) is null`, paymentID,
	).Scan(&lockAcquired); err != nil {
		return fmt.Errorf("locking payment intent for refund: %w", err)
	}
	var paymentUsed bool
	if err := tx.QueryRowContext(ctx,
		`select exists(select 1 from payments where payment_id = $1)`, paymentID,
	).Scan(&paymentUsed); err != nil {
		return fmt.Errorf("checking payment intent before refund: %w", err)
	}
	if paymentUsed {
		return orders.ErrPaymentAlreadyUsed
	}

	var refundStatus string
	err = tx.QueryRowContext(ctx,
		`select status from order_payment_refunds where payment_id = $1 for update`, paymentID,
	).Scan(&refundStatus)
	switch {
	case err == nil && refundStatus == "refunded":
		return orders.ErrPaymentAlreadyUsed
	case err == nil && refundStatus != "pending":
		return fmt.Errorf("unexpected refund status %q", refundStatus)
	case errors.Is(err, sql.ErrNoRows):
		if _, err := tx.ExecContext(ctx, `
			insert into order_payment_refunds (payment_id, status, created_at, updated_at)
			values ($1, 'pending', now(), now())
		`, paymentID); err != nil {
			return fmt.Errorf("recording payment refund claim: %w", err)
		}
	case err != nil:
		return fmt.Errorf("loading payment refund claim: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing payment refund claim: %w", err)
	}

	if err := refundPayment(ctx, paymentID); err != nil {
		return errors.Join(orders.ErrPaymentRefundFailed, fmt.Errorf("refunding payment intent: %w", err))
	}
	result, err := o.DB.ExecContext(ctx, `
		update order_payment_refunds
		set status = 'refunded', updated_at = now()
		where payment_id = $1 and status = 'pending'
	`, paymentID)
	if err != nil {
		return errors.Join(orders.ErrPaymentRefundFailed, fmt.Errorf("recording completed payment refund: %w", err))
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return errors.Join(orders.ErrPaymentRefundFailed, fmt.Errorf("checking completed payment refund: %w", err))
	}
	if affected != 1 {
		var status string
		if err := o.DB.QueryRowContext(ctx,
			`select status from order_payment_refunds where payment_id = $1`, paymentID,
		).Scan(&status); err == nil && status == "refunded" {
			return nil
		}
		return errors.Join(orders.ErrPaymentRefundFailed, errors.New("payment refund claim was not updated"))
	}
	return nil
}

// CreateOrder inserts an order and all related records in one transaction.
func (o *OrdersRepository) CreateOrder(parent context.Context, order models.Order) (*models.Order, error) {
	_, span := observability.StartSpan(parent, "db.orders.create")
	defer span.Finish()
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()

	tx, err := o.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("beginning order creation transaction: %w", err)
	}
	defer tx.Rollback()

	if order.PaymentInfo.ID == "" {
		return nil, orders.ErrInvalidOrderRequest
	}
	var lockAcquired bool
	if err := tx.QueryRowContext(ctx,
		`select pg_advisory_xact_lock(hashtextextended($1, 0)) is null`, order.PaymentInfo.ID,
	).Scan(&lockAcquired); err != nil {
		return nil, fmt.Errorf("locking payment intent: %w", err)
	}
	var paymentUsed bool
	if err := tx.QueryRowContext(ctx,
		`select exists(select 1 from payments where payment_id = $1)
			or exists(select 1 from order_payment_refunds where payment_id = $1)`, order.PaymentInfo.ID,
	).Scan(&paymentUsed); err != nil {
		return nil, fmt.Errorf("checking payment intent reuse: %w", err)
	}
	if paymentUsed {
		return nil, orders.ErrPaymentAlreadyUsed
	}

	quantities, productIDs, err := aggregateQuantities(order.OrderItems)
	if err != nil {
		return nil, err
	}
	trustedItems := make([]*models.Item, 0, len(productIDs))
	var currentTotal int64
	for _, productID := range productIDs {
		var name, image string
		var price, stock int
		err := tx.QueryRowContext(ctx, `
			select p.name, p.price, p.stock,
				coalesce((
					select i.url from images i
					where i.product_id = p.product_id
					order by i.created_at, i.public_id limit 1
				), '')
			from products p
			where p.product_id = $1
			for update
		`, productID).Scan(&name, &price, &stock, &image)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, orders.ErrProductNotFound
		}
		if err != nil {
			return nil, fmt.Errorf("locking product %s for order: %w", productID, err)
		}
		if stock < quantities[productID] {
			return nil, orders.ErrInsufficientStock
		}
		if price < 0 || int64(price) > math.MaxInt64/int64(quantities[productID]) {
			return nil, orders.ErrInvalidOrderRequest
		}
		lineTotal := int64(price) * int64(quantities[productID])
		if currentTotal > int64(math.MaxInt)-lineTotal {
			return nil, orders.ErrInvalidOrderRequest
		}
		currentTotal += lineTotal
		if price != orderItemPrice(order.OrderItems, productID) {
			return nil, orders.ErrOrderPriceChanged
		}
		result, err := tx.ExecContext(ctx, `
			update products set stock = stock - $1
			where product_id = $2 and stock >= $1
		`, quantities[productID], productID)
		if err != nil {
			return nil, fmt.Errorf("reserving product %s stock: %w", productID, err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return nil, fmt.Errorf("checking reserved stock for product %s: %w", productID, err)
		}
		if affected != 1 {
			return nil, orders.ErrInsufficientStock
		}
		trustedItems = append(trustedItems, &models.Item{
			ProductID: productID,
			Name:      name,
			Price:     price,
			Quantity:  quantities[productID],
			Image:     image,
		})
	}
	if currentTotal != int64(order.ItemPrice) {
		return nil, orders.ErrOrderPriceChanged
	}
	order.OrderItems = trustedItems
	order.TaxPrice = 0
	order.ShippingPrice = 0
	order.TotalPrice = order.ItemPrice

	var paidAt any
	if !order.PaidAt.IsZero() {
		paidAt = order.PaidAt
	}
	var deliveredAt any
	if !order.DeliveredAt.IsZero() {
		deliveredAt = order.DeliveredAt
	}
	var paidAtDB, deliveredAtDB sql.NullTime
	err = tx.QueryRowContext(ctx, `
		insert into orders (
			item_price, tax_price, shipping_price, total_price, order_status,
			paid_at, delivered_at, user_id, created_at
		) values ($1, $2, $3, $4, $5, $6, $7, $8, now())
		returning order_id, item_price, tax_price, shipping_price, total_price,
			order_status, paid_at, delivered_at, user_id, created_at
	`,
		order.ItemPrice, order.TaxPrice, order.ShippingPrice, order.TotalPrice,
		order.OrderStatus, paidAt, deliveredAt, order.UserID,
	).Scan(
		&order.OrderID, &order.ItemPrice, &order.TaxPrice, &order.ShippingPrice,
		&order.TotalPrice, &order.OrderStatus, &paidAtDB, &deliveredAtDB,
		&order.UserID, &order.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting order: %w", err)
	}
	if paidAtDB.Valid {
		order.PaidAt = paidAtDB.Time
	}
	if deliveredAtDB.Valid {
		order.DeliveredAt = deliveredAtDB.Time
	}

	shipping := order.ShippingInfo
	shipping.OrderID = order.OrderID
	err = tx.QueryRowContext(ctx, `
		insert into shippings (address, city, phone, postal, country, order_id, created_at)
		values ($1, $2, $3, $4, $5, $6, now())
		returning shipping_id, address, city, phone, postal, country, order_id, created_at
	`, shipping.Address, shipping.City, shipping.PhoneNo, shipping.PostalCode, shipping.Country, order.OrderID).Scan(
		&shipping.ID, &shipping.Address, &shipping.City, &shipping.PhoneNo,
		&shipping.PostalCode, &shipping.Country, &shipping.OrderID, &shipping.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting order shipping: %w", err)
	}
	order.ShippingInfo = shipping

	inputItems := order.OrderItems
	order.OrderItems = make([]*models.Item, 0, len(inputItems))
	for _, input := range inputItems {
		if input == nil {
			return nil, errors.New("order item cannot be nil")
		}
		item := *input
		item.OrderID = order.OrderID
		err = tx.QueryRowContext(ctx, `
			insert into order_items (name, price, quantity, image, product_id, order_id, created_at)
			values ($1, $2, $3, $4, $5, $6, now())
			returning item_id, name, price, quantity, image, product_id, order_id, created_at
		`, item.Name, item.Price, item.Quantity, item.Image, item.ProductID, item.OrderID).Scan(
			&item.ItemID, &item.Name, &item.Price, &item.Quantity, &item.Image,
			&item.ProductID, &item.OrderID, &item.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("inserting order item: %w", err)
		}
		order.OrderItems = append(order.OrderItems, &item)
	}

	payment := order.PaymentInfo
	payment.OrderID = order.OrderID
	err = tx.QueryRowContext(ctx, `
		insert into payments (payment_id, status, order_id, created_at)
		values ($1, $2, $3, now())
		returning payment_id, status, order_id, created_at
	`, payment.ID, payment.Status, payment.OrderID).Scan(
		&payment.ID, &payment.Status, &payment.OrderID, &payment.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting order payment: %w", err)
	}
	order.PaymentInfo = payment

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing order creation: %w", err)
	}
	return &order, nil
}

// FetchOrdersWithDetails returns orders and related records from one query.
// Either filter may be nil; both filters, when present, are applied together.
func (o *OrdersRepository) FetchOrdersWithDetails(parent context.Context, orderID, userID *uuid.UUID, page, pageSize int) ([]*models.Order, int, int64, error) {
	_, span := observability.StartSpan(parent, "db.orders.fetch_with_details")
	defer span.Finish()
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()
	if page < 1 || pageSize < 1 || pageSize > 100 || page-1 > math.MaxInt/pageSize {
		return nil, 0, 0, orders.ErrInvalidPagination
	}

	conditions := make([]string, 0, 2)
	args := make([]any, 0, 4)
	if orderID != nil {
		args = append(args, *orderID)
		conditions = append(conditions, fmt.Sprintf("o.order_id = $%d", len(args)))
	}
	if userID != nil {
		args = append(args, *userID)
		conditions = append(conditions, fmt.Sprintf("o.user_id = $%d", len(args)))
	}
	where := ""
	if len(conditions) > 0 {
		where = " where " + strings.Join(conditions, " and ")
	}
	var totalCount int
	var totalAmount int64
	if err := o.DB.QueryRowContext(ctx,
		"select count(*), coalesce(sum(o.total_price), 0) from orders o"+where, args...,
	).Scan(&totalCount, &totalAmount); err != nil {
		return nil, 0, 0, fmt.Errorf("counting orders: %w", err)
	}

	args = append(args, pageSize, (page-1)*pageSize)
	query := `
		with paged_orders as (
			select o.* from orders o
	` + where + fmt.Sprintf(`
			order by o.created_at desc, o.order_id
			limit $%d offset $%d
		)
		select
			o.order_id, o.item_price, o.tax_price, o.shipping_price, o.total_price,
			o.order_status, o.paid_at, o.delivered_at, o.user_id, o.created_at,
			s.shipping_id, s.address, s.city, s.phone, s.postal, s.country, s.order_id, s.created_at,
			i.item_id, i.name, i.price, i.quantity, i.image, i.product_id, i.order_id, i.created_at,
			p.payment_id, p.status, p.order_id, p.created_at
		from paged_orders o
		join lateral (
			select shipping_id, address, city, phone, postal, country, order_id, created_at
			from shippings
			where order_id = o.order_id
			order by created_at desc, shipping_id
			limit 1
		) s on true
		join order_items i on i.order_id = o.order_id
		join lateral (
			select payment_id, status, order_id, created_at
			from payments
			where order_id = o.order_id
			order by created_at desc, payment_id
			limit 1
		) p on true
		order by o.created_at desc, o.order_id, i.created_at, i.item_id
	`, len(args)-1, len(args))

	rows, err := o.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("querying orders with details: %w", err)
	}
	defer rows.Close()

	ordersByID := make(map[uuid.UUID]*models.Order)
	ordersInOrder := make([]*models.Order, 0)
	for rows.Next() {
		var order models.Order
		var paidAt, deliveredAt sql.NullTime
		var shipping models.Shipping
		var item models.Item
		var payment models.Payment
		if err := rows.Scan(
			&order.OrderID, &order.ItemPrice, &order.TaxPrice, &order.ShippingPrice,
			&order.TotalPrice, &order.OrderStatus, &paidAt, &deliveredAt, &order.UserID, &order.CreatedAt,
			&shipping.ID, &shipping.Address, &shipping.City, &shipping.PhoneNo, &shipping.PostalCode,
			&shipping.Country, &shipping.OrderID, &shipping.CreatedAt,
			&item.ItemID, &item.Name, &item.Price, &item.Quantity, &item.Image, &item.ProductID,
			&item.OrderID, &item.CreatedAt,
			&payment.ID, &payment.Status, &payment.OrderID, &payment.CreatedAt,
		); err != nil {
			return nil, 0, 0, fmt.Errorf("scanning order with details: %w", err)
		}
		if paidAt.Valid {
			order.PaidAt = paidAt.Time
		}
		if deliveredAt.Valid {
			order.DeliveredAt = deliveredAt.Time
		}
		current := ordersByID[order.OrderID]
		if current == nil {
			order.ShippingInfo = shipping
			order.PaymentInfo = payment
			order.OrderItems = make([]*models.Item, 0, 1)
			current = &order
			ordersByID[order.OrderID] = current
			ordersInOrder = append(ordersInOrder, current)
		}
		current.OrderItems = append(current.OrderItems, &item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, 0, fmt.Errorf("iterating orders with details: %w", err)
	}
	return ordersInOrder, totalCount, totalAmount, nil
}

// DeleteOrder deletes an order using the caller's context.
func (o *OrdersRepository) DeleteOrder(parent context.Context, orderID uuid.UUID) error {
	_, span := observability.StartSpan(parent, "db.orders.delete")
	defer span.Finish()
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()
	result, err := o.DB.ExecContext(ctx, `delete from orders where order_id = $1`, orderID)
	if err != nil {
		return fmt.Errorf("deleting order: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking deleted order: %w", err)
	}
	if affected == 0 {
		return orders.ErrOrderNotFound
	}
	return nil
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

// UpdateOrderStatus changes fulfillment status without touching already-reserved stock.
func (o *OrdersRepository) UpdateOrderStatus(parent context.Context, orderID uuid.UUID, status string) error {
	_, span := observability.StartSpan(parent, "db.orders.update_status")
	defer span.Finish()
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()

	tx, err := o.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning order status transaction: %w", err)
	}
	defer tx.Rollback()

	var currentStatus string
	var paidAt sql.NullTime
	if err := tx.QueryRowContext(ctx,
		`select order_status, paid_at from orders where order_id = $1 for update`, orderID,
	).Scan(&currentStatus, &paidAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return orders.ErrOrderNotFound
		}
		return fmt.Errorf("locking order: %w", err)
	}
	if status != "Processing" && status != "Delivered" {
		return orders.ErrInvalidOrderStatus
	}
	if currentStatus == "Delivered" {
		return orders.ErrOrderAlreadyDelivered
	}
	if currentStatus != "Processing" {
		return orders.ErrInvalidOrderStatus
	}
	if !paidAt.Valid {
		return orders.ErrInvalidPayment
	}
	if status == "Processing" {
		return tx.Commit()
	}

	if _, err := tx.ExecContext(ctx, `
		update orders set order_status = $1, delivered_at = now() where order_id = $2
	`, status, orderID); err != nil {
		return fmt.Errorf("updating order status: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing order status transaction: %w", err)
	}
	return nil
}
