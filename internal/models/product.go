package models

import (
	"time"

	"github.com/google/uuid"
)

// Product full model
type Product struct {
	ProductId    uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Price        float64   `json:"price"`
	Description  string    `json:"description"`
	Ratings      int       `json:"ratings"`
	Images       []Images  `json:"images"`
	Category     string    `json:"category"`
	Seller       string    `json:"seller"`
	Stock        int       `json:"stock"`
	NumOfReviews int       `json:"numOfReviews"`
	Reviews      []Reviews `json:"reviews"`
	UserId       uuid.UUID `json:"userId"`
	CreatedAt    time.Time
}

// Images model
type Images struct {
	PublicId  string    `json:"publicId"`
	Url       string    `json:"url"`
	ProductId uuid.UUID `json:"productId"`
	CreatedAt time.Time
}

// Reviews model
type Reviews struct {
	ReviewsId uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Rating    int       `json:"rating"`
	Comment   string    `json:"comment"`
	UserId    uuid.UUID `json:"userId"`
	ProductId uuid.UUID `json:"productId"`
	CreatedAt time.Time
}

type ProdResponse struct {
	Success bool    `json:"success"`
	Token   string  `json:"token,omitempty"`
	Product Product `json:"product"`
}

type GetProd struct {
	Success               bool      `json:"success"`
	ProductCount          int       `json:"productCount"`
	ResPerPage            int       `json:"resPerPage"`
	FilteredProductsCount int       `json:"filteredProductsCount"`
	Products              []Product `json:"products"`
}
