package products

import (
	"mime/multipart"

	"github.com/google/uuid"
	"github.com/jofosuware/go/shopit/internal/models"
)

type ProductUC interface {
	// CreateProduct creates a new product and uploads its images to cloudinary
	CreateProduct(p models.Product, img []*multipart.FileHeader) (*models.ProdResponse, error)

	// GetProducts retrieves products based on a keyword and page number
	GetProducts(keyword string, page int) (*models.GetProd, error)

	// GetAdminProducts retrieves all products for admin use
	GetAdminProducts() ([]*models.Product, error)

	// GetSingleProduct retrieves a single product by its ID
	GetSingleProduct(productId uuid.UUID) (*models.Product, error)

	// UpdateProduct updates a product's details and images by its id
	UpdateProduct(productId uuid.UUID, p models.Product, img []*multipart.File) (*models.ProdResponse, error)

	// DeleteProduct deletes product from the product's table by its id
	DeleteProduct(productId uuid.UUID) error

	// CreateProductReview process product's review and save it into the database
	CreateProductReview(review models.Reviews) error

	// GetProductReviews fetches all reviews for a particular product
	GetProductReviews(productId uuid.UUID) ([]models.Reviews, error)

	// DeleteProductReview deletes a particular review for a product by its id
	DeleteProductReview(productId uuid.UUID, reviewId uuid.UUID) error
}
