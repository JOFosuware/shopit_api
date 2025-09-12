package products

import (
	"github.com/google/uuid"
	"github.com/jofosuware/go/shopit/internal/models"
)

type Repo interface {
	// InsertProduct insert new product into the product table
	InsertProduct(p *models.Product) (models.Product, error)

	// InsertImageUrl inserts product image resource locator into the database
	InsertImageUrl(img *models.Images) (models.Images, error)

	// FetchProductByName fetches product from the product's table by name
	FetchProductByName(keyword string, page int) ([]models.Product, int, error)

	// FetchImageUrlById fetches image url by product id from the database
	FetchImageUrlById(id uuid.UUID) ([]models.Images, error)

	// FetchAllProducts fetches all products from the database
	FetchAllProducts() ([]*models.Product, error)

	// FetchProductById fetches product from the product's table by id
	FetchProductById(id uuid.UUID) (*models.Product, error)

	// DeleteImageUrlById deletes image url by id from the database
	DeleteImageUrlById(id uuid.UUID) error

	// DeleteProductById deletes product from product's table by id
	DeleteProductById(id uuid.UUID) error

	// FetchReviews fetches user reviews for a product
	FetchReviews() ([]models.Reviews, error)

	// UpdateProduct updates a product in the database by id
	UpdateProduct(productId uuid.UUID, p *models.Product) (models.Product, error)

	// InsertReview inserts a review for a product into the reviews table
	InsertReview(r *models.Reviews) error

	// UpdateReview updates reviews with changes by reviewId
	UpdateReview(r *models.Reviews) error

	// FetchReviewById fetches a product review by its ID from the database
	FetchReviewById(productId uuid.UUID) ([]models.Reviews, error)

	// DeleteReviewById deletes a product review by its ID
	DeleteReviewById(productId uuid.UUID) error
}
