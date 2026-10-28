package products

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jofosuware/go/shopit/internal/models"
)

type Repo interface {
	// InsertProduct insert new product into the product table
	InsertProduct(ctx context.Context, p *models.Product) (models.Product, error)

	// InsertImageUrl inserts product image resource locator into the database
	InsertImageUrl(ctx context.Context, img *models.Images) (models.Images, error)

	// FetchProductByName fetches product from the product's table by name
	FetchProductByName(ctx context.Context, keyword string, page int) ([]models.Product, int, int, error)

	// FetchImageUrlById fetches image url by product id from the database
	FetchImageUrlById(ctx context.Context, id uuid.UUID) ([]models.Images, error)
	// FetchImagesByProductIDs fetches image records for multiple products in one query.
	FetchImagesByProductIDs(ctx context.Context, productIDs []uuid.UUID) (map[uuid.UUID][]models.Images, error)

	// FetchAllProducts fetches all products from the database
	FetchAllProducts(ctx context.Context) ([]*models.Product, error)

	// FetchProductById fetches product from the product's table by id
	FetchProductById(ctx context.Context, id uuid.UUID) (*models.Product, error)

	// UpdateProductWithImages atomically updates a product and replaces its image records.
	UpdateProductWithImages(ctx context.Context, productID uuid.UUID, p *models.Product, images []models.Images) (models.Product, []models.Images, error)

	// DeleteProductAndQueueImageCleanup atomically deletes a product and queues its images for cleanup.
	DeleteProductAndQueueImageCleanup(ctx context.Context, productID uuid.UUID, additionalPublicIDs []string) error
	// ScheduleCloudinaryCleanup durably schedules images for Cloudinary deletion.
	ScheduleCloudinaryCleanup(ctx context.Context, publicIDs []string) error
	// StageCloudinaryCleanup schedules uploaded assets for cleanup unless a later transaction adopts them.
	StageCloudinaryCleanup(ctx context.Context, publicIDs []string) error
	// ClaimCloudinaryCleanup leases pending cleanup jobs for processing.
	ClaimCloudinaryCleanup(ctx context.Context, limit int, lease time.Duration) ([]CloudinaryCleanupJob, error)
	// CompleteCloudinaryCleanup removes a successfully processed cleanup job.
	CompleteCloudinaryCleanup(ctx context.Context, publicID string) error
	// RetryCloudinaryCleanup releases a failed job with a future retry time.
	RetryCloudinaryCleanup(ctx context.Context, publicID, failure string, retryAt time.Time) error

	// FetchReviews fetches user reviews for a product
	FetchReviews(ctx context.Context) ([]models.Reviews, error)

	// UpdateProduct updates a product in the database by id
	UpdateProduct(ctx context.Context, productId uuid.UUID, p *models.Product) (models.Product, error)

	// InsertReviewAndUpdateProduct updates a review and its aggregate atomically.
	InsertReviewAndUpdateProduct(ctx context.Context, r *models.Reviews) error
	// DeleteReviewAndUpdateProduct deletes an authorized review and updates aggregates atomically.
	DeleteReviewAndUpdateProduct(ctx context.Context, productID, reviewID, userID uuid.UUID, isAdmin bool) error

	// FetchReviewById fetches a product review by its ID from the database
	FetchReviewById(ctx context.Context, productId uuid.UUID) ([]models.Reviews, error)
}

type CloudinaryCleanupJob struct {
	PublicID string
	Attempts int
}
