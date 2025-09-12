package repository_test

import (
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/jofosuware/go/shopit/internal/models"
	"github.com/jofosuware/go/shopit/internal/products/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInsertProduct(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	defer db.Close()

	repo := repository.NewProdRepository(db)

	p := models.Product{
		Name:         "Test Product",
		Price:        2000,
		Description:  "Test description",
		Ratings:      1,
		Category:     "Home",
		Seller:       "Ebay",
		Stock:        4,
		NumOfReviews: 0,
		UserId:       uuid.UUID{},
	}

	query := `
				insert into products \(name, price, description, ratings, category, seller, stock,
				num_of_reviews, user_id, created_at\) values \(\$1, \$2, \$3, \$4, \$5, \$6, \$7, \$8, \$9, \$10\)
				returning product_id, name, price, description, ratings, category, seller, stock,
				num_of_reviews, user_id, created_at
	`
	t.Run("test product insertion successful", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"product_id", "name", "price", "description", "ratings", "category", "seller",
			"stock", "num_of_reviews", "user_id", "created_at",
		}).AddRow(uuid.UUID{}, p.Name, p.Price, p.Description, p.Ratings, p.Category, p.Seller, p.Stock, p.NumOfReviews, p.UserId,
			time.Now(),
		)

		mock.ExpectQuery(query).WithArgs(p.Name, p.Price, p.Description, p.Ratings, p.Category, p.Seller, p.Stock, p.NumOfReviews, p.UserId,
			sqlmock.AnyArg()).WillReturnRows(rows)

		result, err := repo.InsertProduct(&p)
		require.NoError(t, err)

		assert.Equal(t, p.Name, result.Name)
	})

	t.Run("test product insertion failure", func(t *testing.T) {
		mock.ExpectQuery(query).WithArgs(p.Name, p.Price, p.Description, p.Ratings, p.Category, p.Seller, p.Stock, p.NumOfReviews, p.UserId,
			sqlmock.AnyArg()).WillReturnError(errors.New("database error"))

		_, err := repo.InsertProduct(&p)
		assert.Error(t, err)
	})
}

func TestInsertImageUrl(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)

	defer db.Close()

	repo := repository.NewProdRepository(db)
	var today = time.Now()

	img := models.Images{
		PublicId:  "publicId",
		Url:       "www.testing.com",
		ProductId: uuid.UUID{},
		CreatedAt: today,
	}

	query := `
			insert into images \(public_id, url, product_id, created_at\) 
				values \(\$1, \$2, \$3, \$4\) returning public_id, url, product_id, created_at
	`
	t.Run("Test image insertion successful", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"public_id", "url", "product_id", "created_at"}).AddRow(
			img.PublicId, img.Url, img.ProductId, today,
		)

		mock.ExpectQuery(query).WithArgs(img.PublicId, img.Url, img.ProductId, sqlmock.AnyArg()).WillReturnRows(rows)

		result, err := repo.InsertImageUrl(&img)
		require.NoError(t, err)

		assert.Equal(t, img, result)
	})

	t.Run("Test image insertion failed", func(t *testing.T) {
		mock.ExpectQuery(query).WithArgs(img.PublicId, img.Url, img.ProductId, sqlmock.AnyArg()).
			WillReturnError(errors.New("database error"))

		_, err := repo.InsertImageUrl(&img)
		assert.Error(t, err)
	})
}

func TestFetchProductByName(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)

	defer db.Close()

	repo := repository.NewProdRepository(db)

	t.Run("Success without keyword", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"count"}).AddRow(1)
		mock.ExpectQuery("select count\\(\\*\\) from products").WillReturnRows(rows)

		productRows := sqlmock.NewRows([]string{"product_id", "name", "price", "description", "ratings", "category", "seller", "stock", "num_of_reviews", "user_id", "created_at"}).
			AddRow(uuid.UUID{}, "Test Product", 100.00, "Test Description", 4, "Test Category", "Test Seller", 10, 5, uuid.UUID{}, time.Now())
		mock.ExpectQuery("select \\* from products order by created_at limit").WithArgs(12, 0).WillReturnRows(productRows)

		products, count, err := repo.FetchProductByName("", 1)
		assert.NoError(t, err)
		assert.Len(t, products, 1)
		assert.Equal(t, 1, count)
	})

	t.Run("Success with keyword", func(t *testing.T) {
		keyword := "Test"
		rows := sqlmock.NewRows([]string{"count"}).AddRow(1)
		mock.ExpectQuery("select count\\(\\*\\) from products").WillReturnRows(rows)

		productRows := sqlmock.NewRows([]string{"product_id", "name", "price", "description", "ratings", "category", "seller", "stock", "num_of_reviews", "user_id", "created_at"}).
			AddRow(uuid.UUID{}, "Test Product", 100.00, "Test Description", 4, "Test Category", "Test Seller", 10, 5, uuid.UUID{}, time.Now())
		mock.ExpectQuery("select \\* from products where name ILIKE").WithArgs("%"+keyword+"%", 12, 0).WillReturnRows(productRows)

		products, count, err := repo.FetchProductByName(keyword, 1)
		assert.NoError(t, err)
		assert.Len(t, products, 1)
		assert.Equal(t, 1, count)
	})

	t.Run("Failure on count query", func(t *testing.T) {
		mock.ExpectQuery("select count\\(\\*\\) from products").WillReturnError(errors.New("error"))

		products, count, err := repo.FetchProductByName("", 1)
		assert.Error(t, err)
		assert.Nil(t, products)
		assert.Equal(t, 0, count)
	})

	t.Run("Failure on product query", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"count"}).AddRow(1)
		mock.ExpectQuery("select count\\(\\*\\) from products").WillReturnRows(rows)

		mock.ExpectQuery("select \\* from products order by created_at limit").WithArgs(12, 0).WillReturnError(errors.New("error"))

		products, count, err := repo.FetchProductByName("", 1)
		assert.Error(t, err)
		assert.Nil(t, products)
		assert.Equal(t, 0, count)
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("There were unfulfilled expectations: %s", err)
	}
}

func TestFetchImageUrlById(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := repository.NewProdRepository(db)

	query := "select \\* from images where product_id = \\$1"

	image := models.Images{
		PublicId:  "public_id",
		Url:       "https://example.com/image.jpg",
		ProductId: uuid.UUID{},
		CreatedAt: time.Now(),
	}

	t.Run("Successful fetch", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"public_id", "url", "product_id", "created_at"}).
			AddRow(image.PublicId, image.Url, image.ProductId, image.CreatedAt)

		mock.ExpectQuery(query).WithArgs(image.ProductId).WillReturnRows(rows)

		img, err := repo.FetchImageUrlById(image.ProductId)
		assert.NoError(t, err)
		assert.NotNil(t, img)
	})

	t.Run("Error fetch", func(t *testing.T) {
		mock.ExpectQuery(query).WillReturnError(errors.New("error"))

		img, err := repo.FetchImageUrlById(uuid.UUID{})
		assert.Error(t, err)
		assert.Nil(t, img)
	})
}

func TestFetchAllProducts(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := repository.NewProdRepository(db)

	query := "select \\* from products"

	t.Run("Successful fetch", func(t *testing.T) {
		row := sqlmock.NewRows([]string{"product_id", "name", "price", "description", "ratings", "category", "seller", "stock", "num_of_reviews", "user_id", "created_at"}).
			AddRow(uuid.UUID{}, "Test Product", 100.00, "Test Description", 4, "Test Category", "Test Seller", 10, 5, uuid.UUID{}, time.Now())

		mock.ExpectQuery(query).WillReturnRows(row)

		products, err := repo.FetchAllProducts()
		assert.NoError(t, err)

		assert.NotNil(t, products)
	})

}

func TestFetchProductById(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := repository.NewProdRepository(db)

	query := "select \\* from products where product_id = \\$1"

	t.Run("Successful fetch", func(t *testing.T) {
		row := sqlmock.NewRows([]string{"product_id", "name", "price", "description", "ratings", "category", "seller", "stock", "num_of_reviews", "user_id", "created_at"}).
			AddRow(uuid.UUID{}, "Test Product", 100.00, "Test Description", 4, "Test Category", "Test Seller", 10, 5, uuid.UUID{}, time.Now())

		mock.ExpectQuery(query).WithArgs(uuid.UUID{}).WillReturnRows(row)

		product, err := repo.FetchProductById(uuid.UUID{})
		assert.NoError(t, err)

		assert.NotNil(t, product)
	})
}

func TestDeleteImageUrlById(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	repo := repository.NewProdRepository(db)

	query := "delete from images where product_id = \\$1"

	t.Run("Successful delete", func(t *testing.T) {
		mock.ExpectExec(query).WithArgs(uuid.UUID{}).WillReturnResult(sqlmock.NewResult(0, 1))

		err := repo.DeleteImageUrlById(uuid.UUID{})
		assert.NoError(t, err)
	})
}

func TestDeleteProductById(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	repo := repository.NewProdRepository(db)

	query := "delete from products where product_id = \\$1"

	t.Run("Successful delete", func(t *testing.T) {
		mock.ExpectExec(query).WithArgs(uuid.UUID{}).WillReturnResult(sqlmock.NewResult(0, 1))

		err := repo.DeleteProductById(uuid.UUID{})
		assert.NoError(t, err)

		t.Run("Error delete", func(t *testing.T) {
			mock.ExpectExec(query).WithArgs(uuid.UUID{}).WillReturnError(errors.New("error"))

			err := repo.DeleteProductById(uuid.UUID{})
			assert.Error(t, err)
			assert.Equal(t, "error", err.Error())
		})
	})
}

func TestFetchReviews(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	repo := repository.NewProdRepository(db)

	query := "select \\* from reviews"

	t.Run("Successful fetch", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"review_id", "name", "rating", "comment", "user_id", "product_id", "created_at"}).
			AddRow(uuid.UUID{}, "Test name", 4, "Test Comment", uuid.UUID{}, uuid.UUID{}, time.Now())

		mock.ExpectQuery(query).WillReturnRows(rows)

		reviews, err := repo.FetchReviews()
		assert.NoError(t, err)
		assert.NotNil(t, reviews)
	})
}

func TestUpdateProduct(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	repo := repository.NewProdRepository(db)

	query := "update products set name = \\$1, price = \\$2, description = \\$3, ratings = \\$4, category = \\$5, seller = \\$6, stock = \\$7, num_of_reviews = \\$8, user_id = \\$9, created_at = \\$10 where product_id = \\$11 returning \\*"
	product := &models.Product{
		ProductId:   uuid.UUID{},
		Name:        "Test Product",
		Price:       100.00,
		Description: "Test Description",
		Category:    "Test Category",
		Seller:      "Test Seller",
		Stock:       10,
	}

	t.Run("Successful update", func(t *testing.T) {
		row := sqlmock.NewRows([]string{"product_id", "name", "price", "description", "ratings", "category", "seller", "stock", "num_of_reviews", "user_id", "created_at"}).
			AddRow(product.ProductId, product.Name, product.Price, product.Description, product.Ratings, product.Category, product.Seller, product.Stock, product.NumOfReviews, product.UserId, product.CreatedAt)

		mock.ExpectQuery(query).WithArgs(product.Name, product.Price, product.Description, product.Ratings, product.Category, product.Seller, product.Stock, product.NumOfReviews, product.UserId, product.CreatedAt, product.ProductId).WillReturnRows(row)

		prod, err := repo.UpdateProduct(product.ProductId, product)
		assert.NoError(t, err)

		assert.NotNil(t, prod)
	})
}

func TestInsertReview(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	repo := repository.NewProdRepository(db)

	query := "insert into reviews \\(name, ratings, comment, user_id, product_id, created_at\\) values \\(\\$1, \\$2, \\$3, \\$4, \\$5, \\$6\\)"

	review := &models.Reviews{
		Name:      "Test Name",
		Rating:    4,
		Comment:   "Test Comment",
		UserId:    uuid.UUID{},
		ProductId: uuid.UUID{},
		CreatedAt: time.Now(),
	}

	t.Run("Successful insert", func(t *testing.T) {
		mock.ExpectExec(query).WithArgs(review.Name, review.Rating, review.Comment, review.UserId, review.ProductId, review.CreatedAt).WillReturnResult(sqlmock.NewResult(1, 1))

		err := repo.InsertReview(review)
		assert.NoError(t, err)
	})
}

func TestUpdateReview(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	repo := repository.NewProdRepository(db)

	query := "update reviews set name = \\$1, ratings = \\$2, comment = \\$3, user_id = \\$4, product_id = \\$5, created_at = \\$6 where reviews_id = \\$7"

	review := &models.Reviews{
		ReviewsId: uuid.UUID{},
		Name:      "Test Name",
		Rating:    4,
		Comment:   "Test Comment",
		UserId:    uuid.UUID{},
		ProductId: uuid.UUID{},
		CreatedAt: time.Now(),
	}

	t.Run("Successful update", func(t *testing.T) {
		mock.ExpectExec(query).WithArgs(review.Name, review.Rating, review.Comment, review.UserId, review.ProductId, review.CreatedAt, review.ReviewsId).WillReturnResult(sqlmock.NewResult(1, 1))

		err := repo.UpdateReview(review)
		assert.NoError(t, err)
	})
}

func TestFetchReviewById(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	repo := repository.NewProdRepository(db)

	query := "select \\* from reviews where product_id = \\$1"

	review := &models.Reviews{
		ReviewsId: uuid.UUID{},
		Name:      "Test Name",
		Rating:    4,
		Comment:   "Test Comment",
		UserId:    uuid.UUID{},
		ProductId: uuid.UUID{},
	}

	t.Run("Successful fetch", func(t *testing.T) {
		row := sqlmock.NewRows([]string{"review_id", "name", "rating", "comment", "user_id", "product_id", "created_at"}).
			AddRow(review.ReviewsId, review.Name, review.Rating, review.Comment, review.UserId, review.ProductId, review.CreatedAt)

		mock.ExpectQuery(query).WithArgs(review.ReviewsId).WillReturnRows(row)

		rev, err := repo.FetchReviewById(review.ReviewsId)
		assert.NoError(t, err)

		assert.NotNil(t, rev)
		assert.Equal(t, review.ReviewsId, rev[0].ReviewsId)
	})
}

func TestDeleteReviewById(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	repo := repository.NewProdRepository(db)

	query := "delete from reviews where reviews_id = \\$1"

	review := &models.Reviews{
		ReviewsId: uuid.UUID{},
		Name:      "Test Name",
		Rating:    4,
		Comment:   "Test Comment",
		UserId:    uuid.UUID{},
		ProductId: uuid.UUID{},
	}

	t.Run("Successful delete", func(t *testing.T) {
		mock.ExpectExec(query).WithArgs(review.ReviewsId).WillReturnResult(sqlmock.NewResult(1, 1))

		err := repo.DeleteReviewById(review.ReviewsId)
		assert.NoError(t, err)

	})
}
