// Package repository provides persistence for products.
package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jofosuware/go/shopit/internal/models"
	"github.com/jofosuware/go/shopit/internal/products"
	"github.com/jofosuware/go/shopit/pkg/observability"
)

// ProdRepository handles product-related database operations.
type ProdRepository struct {
	// DB is the database connection.
	DB *sql.DB
}

// NewProdRepository returns a new ProdRepository.
func NewProdRepository(db *sql.DB) *ProdRepository {
	return &ProdRepository{
		DB: db,
	}
}

// InsertProduct inserts a new product into the products table.
func (r *ProdRepository) InsertProduct(parent context.Context, p *models.Product) (models.Product, error) {
	_, span := observability.StartSpan(parent, "db.products.insert_product")
	defer span.Finish()
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()

	var prod models.Product

	query := `
				insert into products (name, price, description, ratings, category, seller, stock,
				num_of_reviews, user_id, created_at) values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
				returning product_id, name, price, description, ratings, category, seller, stock,
				num_of_reviews, user_id, created_at
	`
	err := r.DB.QueryRowContext(ctx, query,
		p.Name,
		p.Price,
		p.Description,
		p.Ratings,
		p.Category,
		p.Seller,
		p.Stock,
		p.NumOfReviews,
		p.UserId,
		time.Now(),
	).Scan(
		&prod.ProductId,
		&prod.Name,
		&prod.Price,
		&prod.Description,
		&prod.Ratings,
		&prod.Category,
		&prod.Seller,
		&prod.Stock,
		&prod.NumOfReviews,
		&prod.UserId,
		&prod.CreatedAt,
	)

	if err != nil {
		return prod, err
	}

	return prod, nil
}

// InsertImageUrl inserts a product image record into the images table.
func (r *ProdRepository) InsertImageUrl(parent context.Context, img *models.Images) (models.Images, error) {
	_, span := observability.StartSpan(parent, "db.products.insert_image")
	defer span.Finish()
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()

	var image models.Images
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return image, fmt.Errorf("beginning image insertion transaction: %w", err)
	}
	defer tx.Rollback()

	err = tx.QueryRowContext(ctx, `
		insert into images (public_id, url, product_id, created_at)
		values ($1, $2, $3, $4)
		returning public_id, url, product_id, created_at
	`,
		img.PublicId,
		img.Url,
		img.ProductId,
		time.Now(),
	).Scan(
		&image.PublicId,
		&image.Url,
		&image.ProductId,
		&image.CreatedAt,
	)
	if err != nil {
		return image, fmt.Errorf("inserting image record: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `delete from cloudinary_cleanup_jobs where public_id = $1`, img.PublicId); err != nil {
		return image, fmt.Errorf("canceling staged image cleanup: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return image, fmt.Errorf("committing image insertion: %w", err)
	}

	return image, nil
}

// FetchProductByName returns products filtered by name (ILIKE) with pagination.
func (r *ProdRepository) FetchProductByName(parent context.Context, keyword string, page int) ([]models.Product, int, int, error) {
	_, span := observability.StartSpan(parent, "db.products.search")
	defer span.Finish()
	if !products.IsValidProductPage(page) {
		return nil, 0, 0, products.ErrInvalidPage
	}
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()

	p := make([]models.Product, 0, products.ProductsPerPage)
	var rows *sql.Rows
	var err error
	var productCount, filteredCount int

	limit := products.ProductsPerPage
	offset := (page - 1) * limit
	if err := r.DB.QueryRowContext(ctx, "select count(*) from products").Scan(&productCount); err != nil {
		return nil, 0, 0, err
	}
	filteredCount = productCount
	query := "select * from products order by created_at desc limit $1 offset $2"

	if keyword != "" {
		query = "select * from products where name ILIKE $1 order by created_at desc limit $2 offset $3"
		if err = r.DB.QueryRowContext(ctx, "select count(*) from products where name ILIKE $1", "%"+keyword+"%").Scan(&filteredCount); err != nil {
			return nil, 0, 0, err
		}
		rows, err = r.DB.QueryContext(ctx, query, "%"+keyword+"%", limit, offset)
	} else {
		rows, err = r.DB.QueryContext(ctx, query, limit, offset)
	}
	if err != nil {
		return nil, 0, 0, err
	}

	defer rows.Close()

	for rows.Next() {
		prod := models.Product{}

		err = rows.Scan(
			&prod.ProductId,
			&prod.Name,
			&prod.Price,
			&prod.Description,
			&prod.Ratings,
			&prod.Category,
			&prod.Seller,
			&prod.Stock,
			&prod.NumOfReviews,
			&prod.UserId,
			&prod.CreatedAt,
		)
		if err != nil {
			return nil, 0, 0, err
		}

		p = append(p, prod)

	}
	if err = rows.Err(); err != nil {
		return nil, 0, 0, err
	}

	return p, productCount, filteredCount, nil
}

// FetchImageUrlById returns image records for a given product ID.
func (r *ProdRepository) FetchImageUrlById(parent context.Context, id uuid.UUID) ([]models.Images, error) {
	_, span := observability.StartSpan(parent, "db.products.get_images")
	defer span.Finish()
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()

	var img []models.Images

	query := "select * from images where product_id = $1"

	rows, err := r.DB.QueryContext(ctx, query, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var image models.Images
		err = rows.Scan(
			&image.PublicId,
			&image.Url,
			&image.ProductId,
			&image.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		img = append(img, image)

		if err = rows.Err(); err != nil {
			return nil, err
		}
	}

	return img, nil
}

// FetchImagesByProductIDs returns images for many products in a single query.
func (r *ProdRepository) FetchImagesByProductIDs(parent context.Context, productIDs []uuid.UUID) (map[uuid.UUID][]models.Images, error) {
	_, span := observability.StartSpan(parent, "db.products.get_images_batch")
	defer span.Finish()
	result := make(map[uuid.UUID][]models.Images, len(productIDs))
	if len(productIDs) == 0 {
		return result, nil
	}

	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()

	placeholders := make([]string, len(productIDs))
	args := make([]interface{}, len(productIDs))
	for i, productID := range productIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = productID
		result[productID] = make([]models.Images, 0)
	}
	query := `select public_id, url, product_id, created_at from images where product_id in (` +
		strings.Join(placeholders, ", ") + `) order by created_at`
	rows, err := r.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("fetching product images: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var image models.Images
		if err := rows.Scan(&image.PublicId, &image.Url, &image.ProductId, &image.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning product image: %w", err)
		}
		result[image.ProductId] = append(result[image.ProductId], image)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading product images: %w", err)
	}
	return result, nil
}

// FetchAllProducts returns all products.
func (r *ProdRepository) FetchAllProducts(parent context.Context) ([]*models.Product, error) {
	_, span := observability.StartSpan(parent, "db.products.list")
	defer span.Finish()
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()

	var products []*models.Product

	query := "select * from products"

	rows, err := r.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	for rows.Next() {
		var prod models.Product
		err = rows.Scan(
			&prod.ProductId,
			&prod.Name,
			&prod.Price,
			&prod.Description,
			&prod.Ratings,
			&prod.Category,
			&prod.Seller,
			&prod.Stock,
			&prod.NumOfReviews,
			&prod.UserId,
			&prod.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		products = append(products, &prod)

		if err = rows.Err(); err != nil {
			return nil, err
		}
	}

	return products, nil
}

// FetchProductById returns a product by its ID.
func (r *ProdRepository) FetchProductById(parent context.Context, id uuid.UUID) (*models.Product, error) {
	_, span := observability.StartSpan(parent, "db.products.get")
	defer span.Finish()
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()

	var prod models.Product

	query := "select * from products where product_id = $1"

	err := r.DB.QueryRowContext(ctx, query, id).Scan(
		&prod.ProductId,
		&prod.Name,
		&prod.Price,
		&prod.Description,
		&prod.Ratings,
		&prod.Category,
		&prod.Seller,
		&prod.Stock,
		&prod.NumOfReviews,
		&prod.UserId,
		&prod.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: %w", products.ErrProductNotFound, err)
		}
		return nil, err
	}

	return &prod, nil
}

// UpdateProductWithImages updates product fields and replaces image records in one transaction.
func (r *ProdRepository) UpdateProductWithImages(parent context.Context, productID uuid.UUID, p *models.Product, images []models.Images) (models.Product, []models.Images, error) {
	_, span := observability.StartSpan(parent, "db.products.update_with_images")
	defer span.Finish()
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()

	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return models.Product{}, nil, fmt.Errorf("beginning product and image update transaction: %w", err)
	}
	defer tx.Rollback()

	if err := lockProduct(ctx, tx, productID, "product update"); err != nil {
		return models.Product{}, nil, err
	}

	rows, err := tx.QueryContext(ctx, `select public_id from images where product_id = $1`, productID)
	if err != nil {
		return models.Product{}, nil, fmt.Errorf("fetching existing product image identifiers: %w", err)
	}
	oldPublicIDs := make([]string, 0)
	for rows.Next() {
		var publicID string
		if err := rows.Scan(&publicID); err != nil {
			rows.Close()
			return models.Product{}, nil, fmt.Errorf("scanning existing product image identifier: %w", err)
		}
		oldPublicIDs = append(oldPublicIDs, publicID)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return models.Product{}, nil, fmt.Errorf("reading existing product image identifiers: %w", err)
	}
	if err := rows.Close(); err != nil {
		return models.Product{}, nil, fmt.Errorf("closing existing product image rows: %w", err)
	}

	var updated models.Product
	err = tx.QueryRowContext(ctx, `
		update products
		set name = $1, price = $2, description = $3, category = $4, seller = $5, stock = $6
		where product_id = $7
		returning product_id, name, price, description, ratings, category, seller, stock,
			num_of_reviews, user_id, created_at
	`,
		p.Name, p.Price, p.Description, p.Category, p.Seller, p.Stock, productID,
	).Scan(
		&updated.ProductId, &updated.Name, &updated.Price, &updated.Description, &updated.Ratings,
		&updated.Category, &updated.Seller, &updated.Stock, &updated.NumOfReviews, &updated.UserId, &updated.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Product{}, nil, fmt.Errorf("%w: %w", products.ErrProductNotFound, err)
		}
		return models.Product{}, nil, fmt.Errorf("updating product fields: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `delete from images where product_id = $1`, productID); err != nil {
		return models.Product{}, nil, fmt.Errorf("deleting existing product image records: %w", err)
	}
	if err := scheduleCloudinaryCleanupTx(ctx, tx, oldPublicIDs); err != nil {
		return models.Product{}, nil, err
	}

	persisted := make([]models.Images, 0, len(images))
	for _, image := range images {
		var saved models.Images
		err := tx.QueryRowContext(ctx, `
			insert into images (public_id, url, product_id, created_at)
			values ($1, $2, $3, $4)
			returning public_id, url, product_id, created_at
		`, image.PublicId, image.Url, productID, time.Now()).Scan(
			&saved.PublicId, &saved.Url, &saved.ProductId, &saved.CreatedAt,
		)
		if err != nil {
			return models.Product{}, nil, fmt.Errorf("inserting replacement image record: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `delete from cloudinary_cleanup_jobs where public_id = $1`, image.PublicId); err != nil {
			return models.Product{}, nil, fmt.Errorf("canceling staged image cleanup: %w", err)
		}
		persisted = append(persisted, saved)
	}
	if err := tx.Commit(); err != nil {
		return models.Product{}, nil, fmt.Errorf("committing product and image update: %w", err)
	}
	updated.Images = persisted
	return updated, persisted, nil
}

// DeleteProductAndQueueImageCleanup atomically removes a product and schedules
// all of its Cloudinary images for asynchronous deletion.
func (r *ProdRepository) DeleteProductAndQueueImageCleanup(parent context.Context, productID uuid.UUID, additionalPublicIDs []string) error {
	_, span := observability.StartSpan(parent, "db.products.delete_with_cleanup")
	defer span.Finish()
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()

	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning product deletion transaction: %w", err)
	}
	defer tx.Rollback()

	if err := lockProduct(ctx, tx, productID, "deletion"); err != nil {
		return err
	}

	rows, err := tx.QueryContext(ctx, `select public_id from images where product_id = $1`, productID)
	if err != nil {
		return fmt.Errorf("fetching product image identifiers: %w", err)
	}
	publicIDs := append([]string(nil), additionalPublicIDs...)
	for rows.Next() {
		var publicID string
		if err := rows.Scan(&publicID); err != nil {
			rows.Close()
			return fmt.Errorf("scanning product image identifier: %w", err)
		}
		publicIDs = append(publicIDs, publicID)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("reading product image identifiers: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("closing product image rows: %w", err)
	}
	if err := scheduleCloudinaryCleanupTx(ctx, tx, publicIDs); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `delete from products where product_id = $1`, productID); err != nil {
		return fmt.Errorf("deleting product: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing product deletion: %w", err)
	}
	return nil
}

// ScheduleCloudinaryCleanup durably records Cloudinary image IDs for retryable cleanup.
func (r *ProdRepository) ScheduleCloudinaryCleanup(parent context.Context, publicIDs []string) error {
	_, span := observability.StartSpan(parent, "db.products.schedule_cleanup")
	defer span.Finish()
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()

	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning Cloudinary cleanup scheduling transaction: %w", err)
	}
	defer tx.Rollback()
	if err := scheduleCloudinaryCleanupTx(ctx, tx, publicIDs); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing Cloudinary cleanup schedule: %w", err)
	}
	return nil
}

// StageCloudinaryCleanup records uploads as pending until they are adopted or
// explicitly confirmed safe to delete.
func (r *ProdRepository) StageCloudinaryCleanup(parent context.Context, publicIDs []string) error {
	_, span := observability.StartSpan(parent, "db.products.stage_cleanup")
	defer span.Finish()
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()

	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning Cloudinary cleanup staging transaction: %w", err)
	}
	defer tx.Rollback()
	for _, publicID := range publicIDs {
		if publicID == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			insert into cloudinary_cleanup_jobs (public_id, cleanup_ready)
			values ($1, false)
			on conflict (public_id) do update
			set cleanup_ready = false, locked_until = null, last_error = ''
		`, publicID); err != nil {
			return fmt.Errorf("staging Cloudinary image cleanup: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing Cloudinary cleanup staging: %w", err)
	}
	return nil
}

func scheduleCloudinaryCleanupTx(ctx context.Context, tx *sql.Tx, publicIDs []string) error {
	seen := make(map[string]struct{}, len(publicIDs))
	for _, publicID := range publicIDs {
		if publicID == "" {
			continue
		}
		if _, exists := seen[publicID]; exists {
			continue
		}
		seen[publicID] = struct{}{}
		if _, err := tx.ExecContext(ctx,
			`insert into cloudinary_cleanup_jobs (public_id, cleanup_ready) values ($1, true)
			 on conflict (public_id) do update
			 set available_at = least(cloudinary_cleanup_jobs.available_at, now()),
			     locked_until = null, last_error = '', cleanup_ready = true`,
			publicID,
		); err != nil {
			return fmt.Errorf("scheduling Cloudinary image cleanup: %w", err)
		}
	}
	return nil
}

// ClaimCloudinaryCleanup atomically leases available jobs across all workers.
func (r *ProdRepository) ClaimCloudinaryCleanup(parent context.Context, limit int, lease time.Duration) ([]products.CloudinaryCleanupJob, error) {
	_, span := observability.StartSpan(parent, "db.products.claim_cleanup")
	defer span.Finish()
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()
	if limit < 1 || lease <= 0 {
		return nil, errors.New("cleanup job limit and lease must be positive")
	}

	rows, err := r.DB.QueryContext(ctx, `
		with jobs as (
			select public_id
			from cloudinary_cleanup_jobs
			where cleanup_ready = true
			  and available_at <= now() and (locked_until is null or locked_until < now())
			order by available_at, created_at
			limit $1
			for update skip locked
		)
		update cloudinary_cleanup_jobs as cleanup
		set locked_until = now() + ($2 * interval '1 second'),
		    attempts = cleanup.attempts + 1
		from jobs
		where cleanup.public_id = jobs.public_id
		returning cleanup.public_id, cleanup.attempts
	`, limit, lease.Seconds())
	if err != nil {
		return nil, fmt.Errorf("claiming Cloudinary cleanup jobs: %w", err)
	}
	defer rows.Close()

	jobs := make([]products.CloudinaryCleanupJob, 0, limit)
	for rows.Next() {
		var job products.CloudinaryCleanupJob
		if err := rows.Scan(&job.PublicID, &job.Attempts); err != nil {
			return nil, fmt.Errorf("scanning Cloudinary cleanup job: %w", err)
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading Cloudinary cleanup jobs: %w", err)
	}
	return jobs, nil
}

// CompleteCloudinaryCleanup removes a job after Cloudinary confirms deletion.
func (r *ProdRepository) CompleteCloudinaryCleanup(parent context.Context, publicID string) error {
	_, span := observability.StartSpan(parent, "db.products.complete_cleanup")
	defer span.Finish()
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()
	if _, err := r.DB.ExecContext(ctx, `delete from cloudinary_cleanup_jobs where public_id = $1`, publicID); err != nil {
		return fmt.Errorf("completing Cloudinary cleanup job: %w", err)
	}
	return nil
}

// RetryCloudinaryCleanup releases a failed job and stores its retry time and error.
func (r *ProdRepository) RetryCloudinaryCleanup(parent context.Context, publicID, failure string, retryAt time.Time) error {
	_, span := observability.StartSpan(parent, "db.products.retry_cleanup")
	defer span.Finish()
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()
	result, err := r.DB.ExecContext(ctx, `
		update cloudinary_cleanup_jobs
		set available_at = $2, locked_until = null, last_error = $3
		where public_id = $1
	`, publicID, retryAt, failure)
	if err != nil {
		return fmt.Errorf("rescheduling Cloudinary cleanup job: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rescheduled Cloudinary cleanup job: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("Cloudinary cleanup job %q no longer exists", publicID)
	}
	return nil
}

// FetchReviews returns all product reviews.
func (r *ProdRepository) FetchReviews(parent context.Context) ([]models.Reviews, error) {
	_, span := observability.StartSpan(parent, "db.products.list_reviews")
	defer span.Finish()
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()

	var reviews []models.Reviews

	query := "select * from reviews"

	rows, err := r.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var review models.Reviews
		err = rows.Scan(
			&review.ReviewsId,
			&review.Name,
			&review.Rating,
			&review.Comment,
			&review.UserId,
			&review.ProductId,
			&review.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		reviews = append(reviews, review)

		if err = rows.Err(); err != nil {
			return nil, err
		}
	}

	return reviews, nil
}

// UpdateProduct updates a product by ID and returns the updated product.
func (r *ProdRepository) UpdateProduct(parent context.Context, productId uuid.UUID, p *models.Product) (models.Product, error) {
	_, span := observability.StartSpan(parent, "db.products.update")
	defer span.Finish()
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()

	query := "update products set name = $1, price = $2, description = $3, category = $4, seller = $5, stock = $6 where product_id = $7 returning *"
	args := []interface{}{p.Name, p.Price, p.Description, p.Category, p.Seller, p.Stock, productId}

	err := r.DB.QueryRowContext(ctx, query, args...).Scan(
		&p.ProductId,
		&p.Name,
		&p.Price,
		&p.Description,
		&p.Ratings,
		&p.Category,
		&p.Seller,
		&p.Stock,
		&p.NumOfReviews,
		&p.UserId,
		&p.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Product{}, fmt.Errorf("%w: %w", products.ErrProductNotFound, err)
		}
		return models.Product{}, err
	}

	return *p, nil
}

// InsertReviewAndUpdateProduct creates a review and recalculates its product's
// aggregate rating in one transaction.
func (r *ProdRepository) InsertReviewAndUpdateProduct(parent context.Context, review *models.Reviews) error {
	_, span := observability.StartSpan(parent, "db.products.insert_review")
	defer span.Finish()
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()

	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning review transaction: %w", err)
	}
	defer tx.Rollback()

	if err = lockProduct(ctx, tx, review.ProductId, "review creation"); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx,
		`insert into reviews (name, ratings, comment, user_id, product_id) values ($1, $2, $3, $4, $5)`,
		review.Name, review.Rating, review.Comment, review.UserId, review.ProductId,
	); err != nil {
		return fmt.Errorf("inserting product review: %w", err)
	}
	if err = updateProductReviewAggregate(ctx, tx, review.ProductId); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("committing review transaction: %w", err)
	}
	return nil
}

// DeleteReviewAndUpdateProduct removes a review owned by the caller, unless the
// caller is an administrator, then recalculates the product aggregate atomically.
func (r *ProdRepository) DeleteReviewAndUpdateProduct(parent context.Context, productID, reviewID, userID uuid.UUID, isAdmin bool) error {
	_, span := observability.StartSpan(parent, "db.products.delete_review")
	defer span.Finish()
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()

	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning review deletion transaction: %w", err)
	}
	defer tx.Rollback()

	if err = lockProduct(ctx, tx, productID, "review deletion"); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx,
		`delete from reviews where reviews_id = $1 and product_id = $2 and ($4 OR user_id = $3)`,
		reviewID, productID, userID, isAdmin,
	)
	if err != nil {
		return fmt.Errorf("deleting product review: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking deleted review: %w", err)
	}
	if rowsAffected == 0 {
		return products.ErrReviewNotFound
	}
	if err = updateProductReviewAggregate(ctx, tx, productID); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("committing review deletion: %w", err)
	}
	return nil
}

func lockProduct(ctx context.Context, tx *sql.Tx, productID uuid.UUID, operation string) error {
	var lockedProductID uuid.UUID
	err := tx.QueryRowContext(ctx,
		`select product_id from products where product_id = $1 for update`,
		productID,
	).Scan(&lockedProductID)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: %w", products.ErrProductNotFound, err)
	}
	if err != nil {
		return fmt.Errorf("locking product for %s: %w", operation, err)
	}
	return nil
}

func updateProductReviewAggregate(ctx context.Context, tx *sql.Tx, productID uuid.UUID) error {
	var rating, reviewCount int
	if err := tx.QueryRowContext(ctx,
		`select coalesce(round(avg(ratings))::integer, 0), count(*) from reviews where product_id = $1`,
		productID,
	).Scan(&rating, &reviewCount); err != nil {
		return fmt.Errorf("calculating product review aggregate: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`update products set ratings = $1, num_of_reviews = $2 where product_id = $3`,
		rating, reviewCount, productID,
	); err != nil {
		return fmt.Errorf("updating product review aggregate: %w", err)
	}
	return nil
}

// FetchReviewById returns reviews for a given product ID.
func (r *ProdRepository) FetchReviewById(parent context.Context, productId uuid.UUID) ([]models.Reviews, error) {
	_, span := observability.StartSpan(parent, "db.products.get_reviews")
	defer span.Finish()
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()

	var reviews []models.Reviews

	query := "select * from reviews where product_id = $1"

	rows, err := r.DB.QueryContext(ctx, query, productId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var review models.Reviews
		err = rows.Scan(
			&review.ReviewsId,
			&review.Name,
			&review.Rating,
			&review.Comment,
			&review.UserId,
			&review.ProductId,
			&review.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		reviews = append(reviews, review)

		if err = rows.Err(); err != nil {
			return nil, err
		}
	}

	return reviews, nil
}
