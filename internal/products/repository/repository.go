package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/jofosuware/go/shopit/internal/models"
)

// ProdRepository is the struct type for Product Repository
type ProdRepository struct {
	DB *sql.DB
}

// NewProdRepository is the constructor for ProdRepository
func NewProdRepository(db *sql.DB) *ProdRepository {
	return &ProdRepository{
		DB: db,
	}
}

// InsertProduct insert new product into the product table
func (r *ProdRepository) InsertProduct(p *models.Product) (models.Product, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
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

// InsertImageUrl inserts product image resource locator into the database
func (r *ProdRepository) InsertImageUrl(img *models.Images) (models.Images, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var image models.Images

	query := `
			insert into images (public_id, url, product_id, created_at) 
				values ($1, $2, $3, $4) returning public_id, url, product_id, created_at
	`

	err := r.DB.QueryRowContext(ctx, query,
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
		return image, err
	}

	return image, nil
}

// FetchProductByName fetches product from the product's table by name
func (r *ProdRepository) FetchProductByName(keyword string, page int) ([]models.Product, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var p []models.Product
	var rows *sql.Rows
	var err error
	var count int

	limit := 12
	offset := (page - 1) * limit

	err = r.DB.QueryRowContext(ctx, "select count(*) from products").Scan(&count)
	if err != nil {
		return p, 0, err
	}

	query := "select * from products order by created_at limit $1 offset $2"

	if keyword != "" {
		query = "select * from products where name ILIKE  $1 order by created_at limit $2 offset $3"
		rows, err = r.DB.QueryContext(ctx, query, "%"+keyword+"%",
			limit, offset,
		)
		if err != nil {
			return p, 0, err
		}
	} else {
		rows, err = r.DB.QueryContext(ctx, query,
			limit, offset,
		)
		if err != nil {
			return p, 0, err
		}
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
			return p, 0, err
		}

		p = append(p, prod)

		if err = rows.Err(); err != nil {
			return p, 0, err
		}
	}

	return p, count, nil
}

// FetchImageUrlById fetches image url by product id from the database
func (r *ProdRepository) FetchImageUrlById(id uuid.UUID) ([]models.Images, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
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

// FetchAllProducts fetches all products from the database
func (r *ProdRepository) FetchAllProducts() ([]*models.Product, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
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

// FetchProductById fetches product from the product's table by id
func (r *ProdRepository) FetchProductById(id uuid.UUID) (*models.Product, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
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
		return nil, err
	}

	return &prod, nil
}

// DeleteImageUrlById deletes image url by id from the database
func (r *ProdRepository) DeleteImageUrlById(id uuid.UUID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := "delete from images where product_id = $1"

	_, err := r.DB.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	return nil
}

// DeleteProductById deletes product from product's table by id
func (r *ProdRepository) DeleteProductById(id uuid.UUID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := "delete from products where product_id = $1"

	_, err := r.DB.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	return nil
}

// FetchReviews fetches user reviews for a product
func (r *ProdRepository) FetchReviews() ([]models.Reviews, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
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

// UpdateProduct updates a product in the database by id
func (r *ProdRepository) UpdateProduct(productId uuid.UUID, p *models.Product) (models.Product, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := "update products set name = $1, price = $2, description = $3, ratings = $4, category = $5, seller = $6, stock = $7, num_of_reviews = $8, user_id = $9, created_at = $10 where product_id = $11 returning *"
	args := []interface{}{p.Name, p.Price, p.Description, p.Ratings, p.Category, p.Seller, p.Stock, p.NumOfReviews, p.UserId, p.CreatedAt, productId}

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
		return models.Product{}, err
	}

	return *p, nil
}

// InsertReview inserts a review for a product into the reviews table
func (r *ProdRepository) InsertReview(review *models.Reviews) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := "insert into reviews (name, ratings, comment, user_id, product_id, created_at) values ($1, $2, $3, $4, $5, $6)"

	_, err := r.DB.ExecContext(ctx, query, review.Name, review.Rating, review.Comment, review.UserId, review.ProductId, review.CreatedAt)
	if err != nil {
		return err
	}

	return nil
}

// UpdateReview updates reviews with changes by reviewId
func (r *ProdRepository) UpdateReview(review *models.Reviews) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := "update reviews set name = $1, ratings = $2, comment = $3, user_id = $4, product_id = $5, created_at = $6 where reviews_id = $7"

	_, err := r.DB.ExecContext(ctx, query, review.Name, review.Rating, review.Comment, review.UserId, review.ProductId, review.CreatedAt, review.ReviewsId)
	if err != nil {
		return err
	}

	return nil
}

// FetchReviewById fetches a product review by its ID from the database
func (r *ProdRepository) FetchReviewById(productId uuid.UUID) ([]models.Reviews, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
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

// DeleteReviewById deletes a product review by its ID
func (r *ProdRepository) DeleteReviewById(reviewId uuid.UUID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := "delete from reviews where reviews_id = $1"

	_, err := r.DB.ExecContext(ctx, query, reviewId)
	if err != nil {
		return err
	}

	return nil
}
