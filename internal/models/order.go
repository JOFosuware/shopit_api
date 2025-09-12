package models

import (
	"time"

	"github.com/google/uuid"
)

type Order struct {
	OrderID       uuid.UUID `json:"id"`
	ShippingInfo  Shipping  `json:"shippingInfo"`
	OrderItems    []*Item   `json:"orderItems"`
	PaymentInfo   Payment   `json:"paymentInfo"`
	UserID        uuid.UUID `json:"userID"`
	PaidAt        time.Time `json:"paidAt"`
	ItemPrice     int       `json:"itemsPrice"`
	TaxPrice      float64       `json:"taxPrice"`
	ShippingPrice int       `json:"shippingPrice"`
	TotalPrice    int       `json:"totalPrice"`
	OrderStatus   string    `json:"orderStatus"`
	DeliveredAt   time.Time `json:"deliveredAt"`
	CreatedAt     time.Time `json:"createdAt"`
}

type Shipping struct {
	ID         uuid.UUID `json:"shippingID,omitempty"`
	Address    string    `json:"address"`
	City       string    `json:"city"`
	PhoneNo    string    `json:"phoneNo"`
	PostalCode string    `json:"postalCode"`
	Country    string    `json:"country"`
	OrderID    uuid.UUID `json:"orderID,omitempty"`
	CreatedAt  time.Time
}

type Item struct {
	ItemID    uuid.UUID `json:"product"`
	Name      string    `json:"name"`
	Price     int       `json:"price"`
	Quantity  int       `json:"quantity"`
	Image     string    `json:"image"`
	ProductID uuid.UUID `json:"productID"`
	OrderID   uuid.UUID `json:"orderID"`
	CreatedAt time.Time
}

type Payment struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	OrderID   uuid.UUID `json:"orderID,omitempty"`
	CreatedAt time.Time
}

type OrderResponse struct {
	Success bool  `json:"success"`
	Order   Order `json:"order,omitempty"`
}
