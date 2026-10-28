package products

import (
	"context"
	"mime/multipart"

	"github.com/google/uuid"
	"github.com/jofosuware/go/shopit/internal/models"
)

type ProductUC interface {
	// CreateProduct creates a new product and uploads its images to cloudinary
	CreateProduct(ctx context.Context, p models.Product, img []*multipart.FileHeader) (*models.ProdResponse, error)

	// GetProducts retrieves products based on a keyword and page number
	GetProducts(ctx context.Context, keyword string, page int) (*models.GetProd, error)

	// GetAdminProducts retrieves all products for admin use
	GetAdminProducts(ctx context.Context) ([]*models.Product, error)

	// GetSingleProduct retrieves a single product by its ID
	GetSingleProduct(ctx context.Context, productId uuid.UUID) (*models.Product, error)

	// UpdateProduct updates a product's details and images by its id
	UpdateProduct(ctx context.Context, productId uuid.UUID, p models.Product, img []*multipart.FileHeader) (*models.ProdResponse, error)

	// DeleteProduct deletes product from the product's table by its id
	DeleteProduct(ctx context.Context, productId uuid.UUID) error

	// CreateProductReview process product's review and save it into the database
	CreateProductReview(ctx context.Context, review models.Reviews) error

	// GetProductReviews fetches all reviews for a particular product
	GetProductReviews(ctx context.Context, productId uuid.UUID) ([]models.Reviews, error)

	// DeleteProductReview deletes a particular review for a product by its id
	DeleteProductReview(ctx context.Context, actor *models.User, productId uuid.UUID, reviewId uuid.UUID) error
}
