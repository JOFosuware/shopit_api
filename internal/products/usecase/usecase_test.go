package usecase_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/jofosuware/go/shopit/internal/models"
	mockProd "github.com/jofosuware/go/shopit/internal/products/mocks"
	"github.com/jofosuware/go/shopit/internal/products/usecase"
	mockCloudinary "github.com/jofosuware/go/shopit/pkg/cloudinary/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// func TestCreateProduct(t *testing.T) {
// 	cld := mockCloudinary.NewCloudUploader(t)
// 	repo := mockProd.NewRepo(t)

// 	u := usecase.NewProductsUC(cld, repo)

// 	t.Run("Create Product successfully", func(t *testing.T) {
// 		formData := url.Values{
// 			"name":        {"test"},
// 			"price":       {"100"},
// 			"description": {"test"},
// 			"category":    {"home"},
// 			"stock":       {"100"},
// 			"seller":      {"test"},
// 			"images":      {"something1.jpg"},
// 		}
// 		payload, ct, err := utils.CreateMultipartForm(formData)
// 		require.NoError(t, err)

// 		req, err := http.NewRequest("POST", "/products", payload)
// 		require.NoError(t, err)

// 		req.Header.Set("Content-Type", ct)
// 		req.ParseMultipartForm(100000)

// 		multipartForm := req.MultipartForm
// 		images := multipartForm.File["images"]
// 		i, _ := utils.ExtractImages(images)
// 		price, _ := strconv.ParseFloat(formData.Get("price"), 64)
// 		stock, _ := strconv.Atoi(formData.Get("stock"))

// 		p := models.Product{
// 			ProductId:   uuid.New(),
// 			Name:        formData.Get("name"),
// 			Price:       price,
// 			Description: formData.Get("description"),
// 			Category:    formData.Get("category"),
// 			Stock:       stock,
// 			Seller:      formData.Get("seller"),
// 		}

// 		var res uploader.UploadResult
// 		var img models.Images

// 		buf := bytes.NewBufferString(formData.Get("images"))

// 		res.PublicID = "test"
// 		res.URL = "test"

// 		img.PublicId = res.PublicID
// 		img.Url = res.URL
// 		img.ProductId = p.ProductId

// 		repo.On("InsertProduct", &p).Return(p, nil)
// 		cld.On("UploadToCloud", "products", buf).Return(&res, nil)
// 		repo.On("InsertImageUrl", &img).Return(img, nil)

// 		resp, err := u.CreateProduct(p, i)
// 		require.NoError(t, err)

// 		assert.NotNil(t, resp)
// 		assert.Equal(t, p.Name, resp.Product.Name)
// 	})
// }

func TestGetProducts(t *testing.T) {
	cld := mockCloudinary.NewCloudUploader(t)
	repo := mockProd.NewRepo(t)

	u := usecase.NewProductsUC(cld, repo)

	t.Run("Get Products successfully", func(t *testing.T) {
		var products []models.Product

		products = append(products, models.Product{
			ProductId:   uuid.New(),
			Name:        "test",
			Price:       100,
			Description: "test",
			Category:    "home",
			Stock:       100,
			Seller:      "test",
		})

		repo.On("FetchProductByName", "", 1).Return(products, 1, nil)
		repo.On("FetchImageUrlById", products[0].ProductId).Return([]models.Images{}, nil)

		res, err := u.GetProducts("", 1)

		require.NoError(t, err)
		assert.NotNil(t, res)
	})
}

func TestGetAdminProducts(t *testing.T) {
	cld := mockCloudinary.NewCloudUploader(t)
	repo := mockProd.NewRepo(t)

	u := usecase.NewProductsUC(cld, repo)

	t.Run("Get Admin Products successfully", func(t *testing.T) {
		repo.On("FetchAllProducts").Return([]*models.Product{}, nil)
		prods, err := u.GetAdminProducts()
		require.NoError(t, err)

		assert.NotNil(t, prods)
	})
}

func TestGetSingleProduct(t *testing.T) {
	cld := mockCloudinary.NewCloudUploader(t)
	repo := mockProd.NewRepo(t)

	u := usecase.NewProductsUC(cld, repo)

	t.Run("Get Single Product successfully", func(t *testing.T) {
		id := uuid.New()

		repo.On("FetchProductById", id).Return(&models.Product{ProductId: id}, nil)
		repo.On("FetchImageUrlById", id).Return([]models.Images{}, nil)
		repo.On("FetchReviewById", id).Return([]models.Reviews{}, nil)

		prod, err := u.GetSingleProduct(id)
		require.NoError(t, err)

		assert.NotNil(t, prod)
	})
}

// func TestUpdateProduct(t *testing.T) {
// 	cld := mockCloudinary.NewCloudUploader(t)
// 	repo := mockProd.NewRepo(t)

// 	u := usecase.NewProductsUC(cld, repo)

// 	t.Run("Update Product successfully", func(t *testing.T) {

// 		res, err := u.UpdateProduct(uuid.New(), models.Product{}, []*multipart.FileHeader{})
// 		require.NoError(t, err)

// 		assert.NotNil(t, res)
// 		assert.Equal(t, true, res.Success)
// 	})
// }

func TestDeleteProduct(t *testing.T) {
	cld := mockCloudinary.NewCloudUploader(t)
	repo := mockProd.NewRepo(t)

	u := usecase.NewProductsUC(cld, repo)

	t.Run("Delete Product successfully", func(t *testing.T) {
		id := uuid.New()
		i := []models.Images{
			{
				PublicId: "test",
			},
		}

		repo.On("FetchImageUrlById", id).Return(i, nil)
		cld.On("Destroy", i[0].PublicId).Return(nil, nil)
		repo.On("DeleteProductById", id).Return(nil)

		err := u.DeleteProduct(id)
		require.NoError(t, err)
	})
}

func TestCreateProductReview(t *testing.T) {
	cld := mockCloudinary.NewCloudUploader(t)
	repo := mockProd.NewRepo(t)

	u := usecase.NewProductsUC(cld, repo)

	t.Run("Create Product Review successfully", func(t *testing.T) {
		review := models.Reviews{
			ProductId: uuid.New(),
			Name:      "test",
			Rating:    5,
			Comment:   "test",
			UserId:    uuid.New(),
		}

		product := models.Product{
			Reviews: []models.Reviews{review},
		}

		repo.On("FetchProductById", review.ProductId).Return(&product, nil)
		repo.On("FetchReviewById", review.ProductId).Return([]models.Reviews{review}, nil)
		repo.On("InsertReview", &review).Return(nil)

		product.NumOfReviews = 1
		product.Ratings = 5
		repo.On("UpdateProduct", review.ProductId, &product).Return(product, nil)

		err := u.CreateProductReview(review)
		require.NoError(t, err)
	})
}

func TestGetProductReviews(t *testing.T) {
	cld := mockCloudinary.NewCloudUploader(t)
	repo := mockProd.NewRepo(t)

	u := usecase.NewProductsUC(cld, repo)

	t.Run("Get Product Reviews successfully", func(t *testing.T) {
		id := uuid.New()

		rvs := models.Reviews{
			ProductId: id,
			Name:      "test",
			Rating:    5,
			Comment:   "test",
			UserId:    uuid.New(),
		}

		repo.On("FetchReviewById", id).Return([]models.Reviews{rvs}, nil)

		reviews, err := u.GetProductReviews(id)
		require.NoError(t, err)

		assert.NotNil(t, reviews)
	})
}

func TestDeleteProductReview(t *testing.T) {
	cld := mockCloudinary.NewCloudUploader(t)
	repo := mockProd.NewRepo(t)

	u := usecase.NewProductsUC(cld, repo)

	t.Run("Delete Product Review successfully", func(t *testing.T) {
		productId := uuid.New()
		reviewId := uuid.New()

		product := models.Product{
			Ratings:      5,
			NumOfReviews: 1,
		}

		reviews := []models.Reviews{
			{
				ProductId: productId,
				Rating:    5,
			},
		}

		repo.On("DeleteReviewById", reviewId).Return(nil)
		repo.On("FetchProductById", productId).Return(&product, nil)
		repo.On("FetchReviewById", productId).Return(reviews, nil)

		var totalRating = 0
		for _, rv := range reviews {
			totalRating += rv.Rating
		}

		product.Ratings = totalRating / len(reviews)
		product.NumOfReviews = len(reviews)
		repo.On("UpdateProduct", productId, &product).Return(models.Product{}, nil)

		err := u.DeleteProductReview(productId, reviewId)
		require.NoError(t, err)
	})
}
