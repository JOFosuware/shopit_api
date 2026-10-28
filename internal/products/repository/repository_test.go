package repository_test

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/jofosuware/go/shopit/internal/models"
	productdomain "github.com/jofosuware/go/shopit/internal/products"
	"github.com/jofosuware/go/shopit/internal/products/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInsertReviewAndUpdateProduct(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := repository.NewProdRepository(db)
	review := &models.Reviews{
		ProductId: uuid.New(),
		UserId:    uuid.New(),
		Name:      "reviewer",
		Rating:    4,
		Comment:   "good",
	}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`select product_id from products where product_id = $1 for update`)).
		WithArgs(review.ProductId).
		WillReturnRows(sqlmock.NewRows([]string{"product_id"}).AddRow(review.ProductId))
	mock.ExpectExec(regexp.QuoteMeta(`insert into reviews (name, ratings, comment, user_id, product_id) values ($1, $2, $3, $4, $5)`)).
		WithArgs(review.Name, review.Rating, review.Comment, review.UserId, review.ProductId).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(regexp.QuoteMeta(`select coalesce(round(avg(ratings))::integer, 0), count(*) from reviews where product_id = $1`)).
		WithArgs(review.ProductId).
		WillReturnRows(sqlmock.NewRows([]string{"rating", "count"}).AddRow(4, 1))
	mock.ExpectExec(regexp.QuoteMeta(`update products set ratings = $1, num_of_reviews = $2 where product_id = $3`)).
		WithArgs(4, 1, review.ProductId).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err = repo.InsertReviewAndUpdateProduct(context.Background(), review)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteLastReviewAndUpdateProduct(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := repository.NewProdRepository(db)
	ctx := context.Background()
	productID, reviewID, userID := uuid.New(), uuid.New(), uuid.New()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`select product_id from products where product_id = $1 for update`)).
		WithArgs(productID).
		WillReturnRows(sqlmock.NewRows([]string{"product_id"}).AddRow(productID))
	mock.ExpectExec(regexp.QuoteMeta(`delete from reviews where reviews_id = $1 and product_id = $2 and ($4 OR user_id = $3)`)).
		WithArgs(reviewID, productID, userID, false).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(`select coalesce(round(avg(ratings))::integer, 0), count(*) from reviews where product_id = $1`)).
		WithArgs(productID).
		WillReturnRows(sqlmock.NewRows([]string{"rating", "count"}).AddRow(0, 0))
	mock.ExpectExec(regexp.QuoteMeta(`update products set ratings = $1, num_of_reviews = $2 where product_id = $3`)).
		WithArgs(0, 0, productID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err = repo.DeleteReviewAndUpdateProduct(ctx, productID, reviewID, userID, false)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteReviewRejectsNonOwner(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := repository.NewProdRepository(db)
	productID, reviewID, userID := uuid.New(), uuid.New(), uuid.New()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`select product_id from products where product_id = $1 for update`)).
		WithArgs(productID).
		WillReturnRows(sqlmock.NewRows([]string{"product_id"}).AddRow(productID))
	mock.ExpectExec(regexp.QuoteMeta(`delete from reviews where reviews_id = $1 and product_id = $2 and ($4 OR user_id = $3)`)).
		WithArgs(reviewID, productID, userID, false).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	err = repo.DeleteReviewAndUpdateProduct(context.Background(), productID, reviewID, userID, false)
	require.ErrorIs(t, err, productdomain.ErrReviewNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}

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

		result, err := repo.InsertProduct(context.Background(), &p)
		require.NoError(t, err)

		assert.Equal(t, p.Name, result.Name)
	})

	t.Run("test product insertion failure", func(t *testing.T) {
		mock.ExpectQuery(query).WithArgs(p.Name, p.Price, p.Description, p.Ratings, p.Category, p.Seller, p.Stock, p.NumOfReviews, p.UserId,
			sqlmock.AnyArg()).WillReturnError(errors.New("database error"))

		_, err := repo.InsertProduct(context.Background(), &p)
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

		mock.ExpectBegin()
		mock.ExpectQuery(query).WithArgs(img.PublicId, img.Url, img.ProductId, sqlmock.AnyArg()).WillReturnRows(rows)
		mock.ExpectExec(regexp.QuoteMeta(`delete from cloudinary_cleanup_jobs where public_id = $1`)).
			WithArgs(img.PublicId).
			WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectCommit()

		result, err := repo.InsertImageUrl(context.Background(), &img)
		require.NoError(t, err)

		assert.Equal(t, img, result)
	})

	t.Run("Test image insertion failed", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectQuery(query).WithArgs(img.PublicId, img.Url, img.ProductId, sqlmock.AnyArg()).
			WillReturnError(errors.New("database error"))
		mock.ExpectRollback()

		_, err := repo.InsertImageUrl(context.Background(), &img)
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
		mock.ExpectQuery("select \\* from products order by created_at desc limit").WithArgs(12, 0).WillReturnRows(productRows)

		products, totalCount, filteredCount, err := repo.FetchProductByName(context.Background(), "", 1)
		assert.NoError(t, err)
		assert.Len(t, products, 1)
		assert.Equal(t, 1, totalCount)
		assert.Equal(t, 1, filteredCount)
	})

	t.Run("Success with keyword", func(t *testing.T) {
		keyword := "Test"
		mock.ExpectQuery("select count\\(\\*\\) from products").
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))
		mock.ExpectQuery("select count\\(\\*\\) from products where name ILIKE \\$1").
			WithArgs("%" + keyword + "%").
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

		productRows := sqlmock.NewRows([]string{"product_id", "name", "price", "description", "ratings", "category", "seller", "stock", "num_of_reviews", "user_id", "created_at"}).
			AddRow(uuid.UUID{}, "Test Product", 100.00, "Test Description", 4, "Test Category", "Test Seller", 10, 5, uuid.UUID{}, time.Now())
		mock.ExpectQuery("select \\* from products where name ILIKE").WithArgs("%"+keyword+"%", 12, 0).WillReturnRows(productRows)

		products, totalCount, filteredCount, err := repo.FetchProductByName(context.Background(), keyword, 1)
		assert.NoError(t, err)
		assert.Len(t, products, 1)
		assert.Equal(t, 3, totalCount)
		assert.Equal(t, 1, filteredCount)
	})

	t.Run("Failure on count query", func(t *testing.T) {
		mock.ExpectQuery("select count\\(\\*\\) from products").WillReturnError(errors.New("error"))

		products, totalCount, filteredCount, err := repo.FetchProductByName(context.Background(), "", 1)
		assert.Error(t, err)
		assert.Nil(t, products)
		assert.Equal(t, 0, totalCount)
		assert.Equal(t, 0, filteredCount)
	})

	t.Run("Failure on product query", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"count"}).AddRow(1)
		mock.ExpectQuery("select count\\(\\*\\) from products").WillReturnRows(rows)

		mock.ExpectQuery("select \\* from products order by created_at desc limit").WithArgs(12, 0).WillReturnError(errors.New("error"))

		products, totalCount, filteredCount, err := repo.FetchProductByName(context.Background(), "", 1)
		assert.Error(t, err)
		assert.Nil(t, products)
		assert.Equal(t, 0, totalCount)
		assert.Equal(t, 0, filteredCount)
	})

	t.Run("Page two uses the next page offset", func(t *testing.T) {
		mock.ExpectQuery("select count\\(\\*\\) from products").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(15))
		mock.ExpectQuery("select \\* from products order by created_at desc limit").
			WithArgs(12, 12).
			WillReturnRows(sqlmock.NewRows([]string{"product_id", "name", "price", "description", "ratings", "category", "seller", "stock", "num_of_reviews", "user_id", "created_at"}))

		gotProducts, totalCount, filteredCount, err := repo.FetchProductByName(context.Background(), "", 2)
		require.NoError(t, err)
		assert.Empty(t, gotProducts)
		assert.Equal(t, 15, totalCount)
		assert.Equal(t, 15, filteredCount)
	})

	t.Run("Rejects invalid page", func(t *testing.T) {
		gotProducts, totalCount, filteredCount, err := repo.FetchProductByName(context.Background(), "", 0)
		require.ErrorIs(t, err, productdomain.ErrInvalidPage)
		assert.Nil(t, gotProducts)
		assert.Zero(t, totalCount)
		assert.Zero(t, filteredCount)
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("There were unfulfilled expectations: %s", err)
	}
}

func TestScheduleAndClaimCloudinaryCleanup(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := repository.NewProdRepository(db)

	mock.ExpectBegin()
	mock.ExpectExec(`(?s)insert into cloudinary_cleanup_jobs \(public_id, cleanup_ready\).*values \(\$1, true\).*cleanup_ready = true`).
		WithArgs("stale-image").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	require.NoError(t, repo.ScheduleCloudinaryCleanup(context.Background(), []string{"stale-image", ""}))

	mock.ExpectBegin()
	mock.ExpectExec(`(?s)insert into cloudinary_cleanup_jobs \(public_id, cleanup_ready\).*values \(\$1, false\).*on conflict \(public_id\) do update.*cleanup_ready = false`).
		WithArgs("staged-image").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	require.NoError(t, repo.StageCloudinaryCleanup(context.Background(), []string{"staged-image"}))

	mock.ExpectQuery(`(?s)with jobs as .*cleanup_ready = true.*`).WithArgs(5, 60.0).
		WillReturnRows(sqlmock.NewRows([]string{"public_id", "attempts"}).AddRow("stale-image", 1))
	jobs, err := repo.ClaimCloudinaryCleanup(context.Background(), 5, time.Minute)
	require.NoError(t, err)
	require.Equal(t, []productdomain.CloudinaryCleanupJob{{PublicID: "stale-image", Attempts: 1}}, jobs)

	mock.ExpectExec(regexp.QuoteMeta(`delete from cloudinary_cleanup_jobs where public_id = $1`)).
		WithArgs("stale-image").
		WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.CompleteCloudinaryCleanup(context.Background(), "stale-image"))

	retryAt := time.Now().Add(time.Minute)
	mock.ExpectExec(regexp.QuoteMeta(`
		update cloudinary_cleanup_jobs
		set available_at = $2, locked_until = null, last_error = $3
		where public_id = $1
	`)).
		WithArgs("staged-image", retryAt, "Cloudinary unavailable").
		WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.RetryCloudinaryCleanup(context.Background(), "staged-image", "Cloudinary unavailable", retryAt))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteProductQueuesImagesInSameTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := repository.NewProdRepository(db)
	productID := uuid.New()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`select product_id from products where product_id = $1 for update`)).
		WithArgs(productID).
		WillReturnRows(sqlmock.NewRows([]string{"product_id"}).AddRow(productID))
	mock.ExpectQuery(regexp.QuoteMeta(`select public_id from images where product_id = $1`)).
		WithArgs(productID).
		WillReturnRows(sqlmock.NewRows([]string{"public_id"}).AddRow("stored-image"))
	mock.ExpectExec(`insert into cloudinary_cleanup_jobs \(public_id, cleanup_ready\)`).
		WithArgs("uploaded-but-not-stored").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`insert into cloudinary_cleanup_jobs \(public_id, cleanup_ready\)`).
		WithArgs("stored-image").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`delete from products where product_id = $1`)).
		WithArgs(productID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err = repo.DeleteProductAndQueueImageCleanup(context.Background(), productID, []string{"uploaded-but-not-stored"})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateProductWithImagesCommitsFieldsImagesAndCleanupAtomically(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := repository.NewProdRepository(db)
	productID := uuid.New()
	product := &models.Product{
		Name:        "Updated product",
		Price:       42,
		Description: "Updated description",
		Category:    "home",
		Seller:      "seller",
		Stock:       8,
	}
	image := models.Images{
		PublicId: "replacement-image",
		Url:      "https://example.test/replacement.jpg",
	}
	createdAt := time.Now()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`select product_id from products where product_id = $1 for update`)).
		WithArgs(productID).
		WillReturnRows(sqlmock.NewRows([]string{"product_id"}).AddRow(productID))
	mock.ExpectQuery(regexp.QuoteMeta(`select public_id from images where product_id = $1`)).
		WithArgs(productID).
		WillReturnRows(sqlmock.NewRows([]string{"public_id"}).AddRow("old-image"))
	mock.ExpectQuery(regexp.QuoteMeta(`
		update products
		set name = $1, price = $2, description = $3, category = $4, seller = $5, stock = $6
		where product_id = $7
		returning product_id, name, price, description, ratings, category, seller, stock,
			num_of_reviews, user_id, created_at
	`)).
		WithArgs(product.Name, product.Price, product.Description, product.Category, product.Seller, product.Stock, productID).
		WillReturnRows(sqlmock.NewRows([]string{
			"product_id", "name", "price", "description", "ratings", "category", "seller",
			"stock", "num_of_reviews", "user_id", "created_at",
		}).AddRow(productID, product.Name, product.Price, product.Description, 3, product.Category,
			product.Seller, product.Stock, 4, uuid.New(), createdAt))
	mock.ExpectExec(regexp.QuoteMeta(`delete from images where product_id = $1`)).
		WithArgs(productID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`insert into cloudinary_cleanup_jobs \(public_id, cleanup_ready\)`).
		WithArgs("old-image").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(`
			insert into images (public_id, url, product_id, created_at)
			values ($1, $2, $3, $4)
			returning public_id, url, product_id, created_at
		`)).
		WithArgs(image.PublicId, image.Url, productID, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"public_id", "url", "product_id", "created_at"}).
			AddRow(image.PublicId, image.Url, productID, createdAt))
	mock.ExpectExec(regexp.QuoteMeta(`delete from cloudinary_cleanup_jobs where public_id = $1`)).
		WithArgs(image.PublicId).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	updated, images, err := repo.UpdateProductWithImages(context.Background(), productID, product, []models.Images{image})
	require.NoError(t, err)
	require.Equal(t, "Updated product", updated.Name)
	require.Len(t, images, 1)
	require.Equal(t, image.PublicId, images[0].PublicId)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateProductWithImagesRollsBackImageFailure(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := repository.NewProdRepository(db)
	productID := uuid.New()
	product := &models.Product{Name: "Updated product"}
	image := models.Images{PublicId: "replacement-image", Url: "https://example.test/replacement.jpg"}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`select product_id from products where product_id = $1 for update`)).
		WithArgs(productID).
		WillReturnRows(sqlmock.NewRows([]string{"product_id"}).AddRow(productID))
	mock.ExpectQuery(regexp.QuoteMeta(`select public_id from images where product_id = $1`)).
		WithArgs(productID).
		WillReturnRows(sqlmock.NewRows([]string{"public_id"}).AddRow("old-image"))
	mock.ExpectQuery(regexp.QuoteMeta(`
		update products
		set name = $1, price = $2, description = $3, category = $4, seller = $5, stock = $6
		where product_id = $7
		returning product_id, name, price, description, ratings, category, seller, stock,
			num_of_reviews, user_id, created_at
	`)).
		WithArgs(product.Name, product.Price, product.Description, product.Category, product.Seller, product.Stock, productID).
		WillReturnRows(sqlmock.NewRows([]string{
			"product_id", "name", "price", "description", "ratings", "category", "seller",
			"stock", "num_of_reviews", "user_id", "created_at",
		}).AddRow(productID, product.Name, product.Price, product.Description, 0, product.Category,
			product.Seller, product.Stock, 0, uuid.New(), time.Now()))
	mock.ExpectExec(regexp.QuoteMeta(`delete from images where product_id = $1`)).
		WithArgs(productID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`insert into cloudinary_cleanup_jobs \(public_id, cleanup_ready\)`).
		WithArgs("old-image").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(`
			insert into images (public_id, url, product_id, created_at)
			values ($1, $2, $3, $4)
			returning public_id, url, product_id, created_at
		`)).
		WithArgs(image.PublicId, image.Url, productID, sqlmock.AnyArg()).
		WillReturnError(errors.New("image insert failed"))
	mock.ExpectRollback()

	_, _, err = repo.UpdateProductWithImages(context.Background(), productID, product, []models.Images{image})
	require.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFetchImagesByProductIDsBatchesProductImages(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := repository.NewProdRepository(db)

	productID1, productID2 := uuid.New(), uuid.New()
	createdAt := time.Now()
	mock.ExpectQuery(regexp.QuoteMeta(`select public_id, url, product_id, created_at from images where product_id in ($1, $2) order by created_at`)).
		WithArgs(productID1, productID2).
		WillReturnRows(sqlmock.NewRows([]string{"public_id", "url", "product_id", "created_at"}).
			AddRow("image-1", "https://example.test/1.jpg", productID1, createdAt).
			AddRow("image-2", "https://example.test/2.jpg", productID1, createdAt).
			AddRow("image-3", "https://example.test/3.jpg", productID2, createdAt))

	images, err := repo.FetchImagesByProductIDs(context.Background(), []uuid.UUID{productID1, productID2})
	require.NoError(t, err)
	require.Len(t, images[productID1], 2)
	require.Len(t, images[productID2], 1)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFetchImagesByProductIDsEmptyInputDoesNotQuery(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := repository.NewProdRepository(db)

	images, err := repo.FetchImagesByProductIDs(context.Background(), nil)
	require.NoError(t, err)
	require.Empty(t, images)
	require.NoError(t, mock.ExpectationsWereMet())
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

		img, err := repo.FetchImageUrlById(context.Background(), image.ProductId)
		assert.NoError(t, err)
		assert.NotNil(t, img)
	})

	t.Run("Error fetch", func(t *testing.T) {
		mock.ExpectQuery(query).WillReturnError(errors.New("error"))

		img, err := repo.FetchImageUrlById(context.Background(), uuid.UUID{})
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

		products, err := repo.FetchAllProducts(context.Background())
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

		product, err := repo.FetchProductById(context.Background(), uuid.UUID{})
		assert.NoError(t, err)

		assert.NotNil(t, product)
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

		reviews, err := repo.FetchReviews(context.Background())
		assert.NoError(t, err)
		assert.NotNil(t, reviews)
	})
}

func TestUpdateProduct(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	repo := repository.NewProdRepository(db)

	query := "update products set name = \\$1, price = \\$2, description = \\$3, category = \\$4, seller = \\$5, stock = \\$6 where product_id = \\$7 returning \\*"
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

		mock.ExpectQuery(query).WithArgs(product.Name, product.Price, product.Description, product.Category, product.Seller, product.Stock, product.ProductId).WillReturnRows(row)

		prod, err := repo.UpdateProduct(context.Background(), product.ProductId, product)
		assert.NoError(t, err)

		assert.NotNil(t, prod)
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

		rev, err := repo.FetchReviewById(context.Background(), review.ReviewsId)
		assert.NoError(t, err)

		assert.NotNil(t, rev)
		assert.Equal(t, review.ReviewsId, rev[0].ReviewsId)
	})
}
