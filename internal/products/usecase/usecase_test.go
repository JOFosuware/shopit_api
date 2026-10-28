package usecase_test

import (
	"bytes"
	"context"
	"errors"
	"mime/multipart"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cloudinary/cloudinary-go/api/uploader"
	"github.com/google/uuid"
	"github.com/jofosuware/go/shopit/internal/models"
	productdomain "github.com/jofosuware/go/shopit/internal/products"
	mockProd "github.com/jofosuware/go/shopit/internal/products/mocks"
	"github.com/jofosuware/go/shopit/internal/products/usecase"
	mockCloudinary "github.com/jofosuware/go/shopit/pkg/cloudinary/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func expectProductImageUpload(t *testing.T, cld *mockCloudinary.CloudUploader, repo *mockProd.Repo, ctx context.Context, imageURL string) {
	t.Helper()
	repo.On("StageCloudinaryCleanup", ctx, mock.MatchedBy(func(ids []string) bool {
		return len(ids) >= 1 && strings.HasPrefix(ids[len(ids)-1], "products/")
	})).Return(nil).Once()
	cld.On(
		"UploadToCloudWithPublicID",
		mock.MatchedBy(func(uploadCtx context.Context) bool {
			deadline, ok := uploadCtx.Deadline()
			return ok && time.Until(deadline) <= 5*time.Minute && time.Until(deadline) > 0
		}),
		"products",
		mock.MatchedBy(func(publicID string) bool {
			_, err := uuid.Parse(publicID)
			return err == nil
		}),
		mock.Anything,
	).Run(func(args mock.Arguments) {
		repo.AssertCalled(t, "StageCloudinaryCleanup", ctx, []string{"products/" + args.String(2)})
	}).Return(func(_ context.Context, folder, publicID string, _ interface{}) (*uploader.UploadResult, error) {
		return &uploader.UploadResult{
			PublicID: folder + "/" + publicID,
			URL:      imageURL,
		}, nil
	})
}

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
		ctx := context.Background()
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

		repo.On("FetchProductByName", ctx, "", 1).Return(products, 3, 1, nil)
		repo.On("FetchImagesByProductIDs", ctx, []uuid.UUID{products[0].ProductId}).
			Return(map[uuid.UUID][]models.Images{products[0].ProductId: {}}, nil)

		res, err := u.GetProducts(ctx, "", 1)

		require.NoError(t, err)
		assert.NotNil(t, res)
		assert.Equal(t, 3, res.ProductCount)
		assert.Equal(t, 1, res.FilteredProductsCount)
		assert.Equal(t, 12, res.ResPerPage)
	})
}

func TestGetAdminProducts(t *testing.T) {
	cld := mockCloudinary.NewCloudUploader(t)
	repo := mockProd.NewRepo(t)

	u := usecase.NewProductsUC(cld, repo)

	t.Run("Get Admin Products successfully", func(t *testing.T) {
		ctx := context.Background()
		repo.On("FetchAllProducts", ctx).Return([]*models.Product{}, nil)
		prods, err := u.GetAdminProducts(ctx)
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
		ctx := context.Background()

		repo.On("FetchProductById", ctx, id).Return(&models.Product{ProductId: id}, nil)
		repo.On("FetchImageUrlById", ctx, id).Return([]models.Images{}, nil)
		repo.On("FetchReviewById", ctx, id).Return([]models.Reviews{}, nil)

		prod, err := u.GetSingleProduct(ctx, id)
		require.NoError(t, err)

		assert.NotNil(t, prod)
	})
}

func TestUpdateProductWithoutReplacementImagesKeepsExistingImages(t *testing.T) {
	cld := mockCloudinary.NewCloudUploader(t)
	repo := mockProd.NewRepo(t)
	u := usecase.NewProductsUC(cld, repo)

	ctx := context.Background()
	productID := uuid.New()
	product := models.Product{Name: "Updated product", ProductId: productID}
	existingImages := []models.Images{{PublicId: "existing-image", ProductId: productID}}
	repo.On("UpdateProduct", ctx, productID, &product).Return(product, nil)
	repo.On("FetchImageUrlById", ctx, productID).Return(existingImages, nil)

	response, err := u.UpdateProduct(ctx, productID, product, nil)

	require.NoError(t, err)
	require.NotNil(t, response)
	require.Equal(t, existingImages, response.Product.Images)
	repo.AssertNotCalled(t, "UpdateProductWithImages", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestUpdateProductWithImagesUsesAtomicRepositoryOperation(t *testing.T) {
	cld := mockCloudinary.NewCloudUploader(t)
	repo := mockProd.NewRepo(t)
	u := usecase.NewProductsUC(cld, repo)

	productID := uuid.New()
	product := models.Product{Name: "Updated product", ProductId: productID}
	uploadedImage := models.Images{
		Url:       "https://example.test/replacement.jpg",
		ProductId: productID,
	}
	payload := &bytes.Buffer{}
	writer := multipart.NewWriter(payload)
	file, err := writer.CreateFormFile("images", "image.jpg")
	require.NoError(t, err)
	_, err = file.Write([]byte("image contents"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	request := httptest.NewRequest("PUT", "/", payload)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	require.NoError(t, request.ParseMultipartForm(1<<20))
	imageHeaders := request.MultipartForm.File["images"]
	ctx := context.Background()

	expectProductImageUpload(t, cld, repo, ctx, uploadedImage.Url)
	repo.On("UpdateProductWithImages", ctx, productID, &product, mock.MatchedBy(func(images []models.Images) bool {
		return len(images) == 1 && strings.HasPrefix(images[0].PublicId, "products/") &&
			images[0].Url == uploadedImage.Url && images[0].ProductId == productID
	})).Return(func(_ context.Context, _ uuid.UUID, product *models.Product, images []models.Images) (models.Product, []models.Images, error) {
		return *product, images, nil
	})

	response, err := u.UpdateProduct(ctx, productID, product, imageHeaders)

	require.NoError(t, err)
	require.NotNil(t, response)
	require.Len(t, response.Product.Images, 1)
	require.True(t, strings.HasPrefix(response.Product.Images[0].PublicId, "products/"))
	require.Equal(t, uploadedImage.Url, response.Product.Images[0].Url)
	repo.AssertNotCalled(t, "UpdateProduct", mock.Anything, mock.Anything, mock.Anything)
}

func TestUpdateProductFailureLeavesStagedCleanupForAmbiguousCommit(t *testing.T) {
	cld := mockCloudinary.NewCloudUploader(t)
	repo := mockProd.NewRepo(t)
	u := usecase.NewProductsUC(cld, repo)

	productID := uuid.New()
	product := models.Product{Name: "Updated product", ProductId: productID}
	uploadedImage := models.Images{
		Url:       "https://example.test/replacement.jpg",
		ProductId: productID,
	}
	payload := &bytes.Buffer{}
	writer := multipart.NewWriter(payload)
	file, err := writer.CreateFormFile("images", "image.jpg")
	require.NoError(t, err)
	_, err = file.Write([]byte("image contents"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	request := httptest.NewRequest("PUT", "/", payload)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	require.NoError(t, request.ParseMultipartForm(1<<20))
	imageHeaders := request.MultipartForm.File["images"]
	ctx := context.Background()

	expectProductImageUpload(t, cld, repo, ctx, uploadedImage.Url)
	repo.On("UpdateProductWithImages", ctx, productID, &product, mock.MatchedBy(func(images []models.Images) bool {
		return len(images) == 1 && strings.HasPrefix(images[0].PublicId, "products/")
	})).Return(models.Product{}, []models.Images(nil), errors.New("commit result is unknown"))

	response, err := u.UpdateProduct(ctx, productID, product, imageHeaders)

	require.Error(t, err)
	assert.Nil(t, response)
	repo.AssertNotCalled(t, "ScheduleCloudinaryCleanup", mock.Anything, mock.Anything)
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
		repo.On("DeleteProductAndQueueImageCleanup", mock.Anything, id, mock.Anything).Return(nil)

		err := u.DeleteProduct(context.Background(), id)
		require.NoError(t, err)
	})

	t.Run("product deletion succeeds after durable image-cleanup scheduling", func(t *testing.T) {
		id := uuid.New()
		repo.On("DeleteProductAndQueueImageCleanup", mock.Anything, id, []string(nil)).Return(nil)

		err := u.DeleteProduct(context.Background(), id)
		require.NoError(t, err)
		repo.AssertCalled(t, "DeleteProductAndQueueImageCleanup", mock.Anything, id, mock.Anything)
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

		ctx := context.Background()
		repo.On("InsertReviewAndUpdateProduct", ctx, &review).Return(nil)

		err := u.CreateProductReview(ctx, review)
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

		repo.On("FetchReviewById", context.Background(), id).Return([]models.Reviews{rvs}, nil)

		reviews, err := u.GetProductReviews(context.Background(), id)
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
		ctx := context.Background()
		actor := &models.User{ID: uuid.New(), Role: "user"}

		repo.On("DeleteReviewAndUpdateProduct", ctx, productId, reviewId, actor.ID, false).Return(nil)

		err := u.DeleteProductReview(ctx, actor, productId, reviewId)
		require.NoError(t, err)
	})
}

func TestCreateProductCleansUpProductWhenImageUploadFails(t *testing.T) {
	cld := mockCloudinary.NewCloudUploader(t)
	repo := mockProd.NewRepo(t)
	u := usecase.NewProductsUC(cld, repo)

	product := models.Product{
		ProductId: uuid.New(),
		Name:      "test product",
	}
	payload := &bytes.Buffer{}
	writer := multipart.NewWriter(payload)
	file, err := writer.CreateFormFile("images", "image.jpg")
	require.NoError(t, err)
	_, err = file.Write([]byte("image contents"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	request := httptest.NewRequest("POST", "/", payload)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	require.NoError(t, request.ParseMultipartForm(1<<20))
	imageHeaders := request.MultipartForm.File["images"]

	uploadErr := errors.New("Cloudinary unavailable")
	ctx := context.Background()
	var ambiguousPublicID string
	repo.On("InsertProduct", ctx, &product).Return(product, nil)
	repo.On("StageCloudinaryCleanup", ctx, mock.MatchedBy(func(ids []string) bool {
		return len(ids) == 1 && strings.HasPrefix(ids[0], "products/")
	})).Run(func(args mock.Arguments) {
		ambiguousPublicID = args.Get(1).([]string)[0]
	}).Return(nil)
	cld.On("UploadToCloudWithPublicID", mock.Anything, "products", mock.Anything, mock.Anything).
		Return((*uploader.UploadResult)(nil), uploadErr)
	repo.On("ScheduleCloudinaryCleanup", mock.Anything, mock.Anything).Return(nil)
	repo.On("DeleteProductAndQueueImageCleanup", mock.Anything, product.ProductId, mock.MatchedBy(func(ids []string) bool {
		return len(ids) == 0
	})).Return(nil)

	response, err := u.CreateProduct(ctx, product, imageHeaders)
	require.ErrorIs(t, err, uploadErr)
	require.ErrorIs(t, err, productdomain.ErrImageUploadFailed)
	assert.Nil(t, response)
	require.NotEmpty(t, ambiguousPublicID)
	repo.AssertNotCalled(t, "ScheduleCloudinaryCleanup", mock.Anything, mock.MatchedBy(func(ids []string) bool {
		return len(ids) > 0 && ids[0] == ambiguousPublicID
	}))
	repo.AssertCalled(t, "DeleteProductAndQueueImageCleanup", mock.Anything, product.ProductId, []string{})
}

func TestCreateProductStagesUploadedAssetsUntilImageRecordIsSaved(t *testing.T) {
	cld := mockCloudinary.NewCloudUploader(t)
	repo := mockProd.NewRepo(t)
	u := usecase.NewProductsUC(cld, repo)

	product := models.Product{ProductId: uuid.New(), Name: "test product"}
	payload := &bytes.Buffer{}
	writer := multipart.NewWriter(payload)
	file, err := writer.CreateFormFile("images", "image.jpg")
	require.NoError(t, err)
	_, err = file.Write([]byte("image contents"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	request := httptest.NewRequest("POST", "/", payload)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	require.NoError(t, request.ParseMultipartForm(1<<20))
	imageHeaders := request.MultipartForm.File["images"]

	uploadedImage := models.Images{
		Url:       "https://example.test/image.jpg",
		ProductId: product.ProductId,
	}
	ctx := context.Background()
	repo.On("InsertProduct", ctx, &product).Return(product, nil)
	expectProductImageUpload(t, cld, repo, ctx, uploadedImage.Url)
	repo.On("InsertImageUrl", ctx, mock.MatchedBy(func(image *models.Images) bool {
		return image != nil && strings.HasPrefix(image.PublicId, "products/") &&
			image.Url == uploadedImage.Url && image.ProductId == product.ProductId
	})).Return(func(_ context.Context, image *models.Images) (models.Images, error) {
		return *image, nil
	})

	response, err := u.CreateProduct(ctx, product, imageHeaders)
	require.NoError(t, err)
	require.NotNil(t, response)
	require.Len(t, response.Product.Images, 1)
	assert.True(t, strings.HasPrefix(response.Product.Images[0].PublicId, "products/"))
}

func TestCreateProductRenewsAllStagedImagesBetweenUploads(t *testing.T) {
	cld := mockCloudinary.NewCloudUploader(t)
	repo := mockProd.NewRepo(t)
	u := usecase.NewProductsUC(cld, repo)

	product := models.Product{ProductId: uuid.New(), Name: "test product"}
	payload := &bytes.Buffer{}
	writer := multipart.NewWriter(payload)
	for _, name := range []string{"first.jpg", "second.jpg"} {
		file, err := writer.CreateFormFile("images", name)
		require.NoError(t, err)
		_, err = file.Write([]byte(name))
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())

	request := httptest.NewRequest("POST", "/", payload)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	require.NoError(t, request.ParseMultipartForm(1<<20))
	imageHeaders := request.MultipartForm.File["images"]

	ctx := context.Background()
	repo.On("InsertProduct", ctx, &product).Return(product, nil)
	stagedIDs := make([][]string, 0, len(imageHeaders))
	repo.On("StageCloudinaryCleanup", ctx, mock.Anything).
		Run(func(args mock.Arguments) {
			ids := args.Get(1).([]string)
			stagedIDs = append(stagedIDs, append([]string(nil), ids...))
		}).
		Return(nil).
		Times(2*len(imageHeaders) - 1)

	uploadedIDs := make([]string, 0, len(imageHeaders))
	cld.On(
		"UploadToCloudWithPublicID",
		mock.MatchedBy(func(uploadCtx context.Context) bool {
			deadline, ok := uploadCtx.Deadline()
			return ok && time.Until(deadline) <= 5*time.Minute && time.Until(deadline) > 0
		}),
		"products",
		mock.Anything,
		mock.Anything,
	).Run(func(args mock.Arguments) {
		require.Len(t, stagedIDs, len(uploadedIDs)+1)
		expectedID := "products/" + args.String(2)
		require.Equal(t, expectedID, stagedIDs[len(stagedIDs)-1][len(stagedIDs[len(stagedIDs)-1])-1])
		uploadedIDs = append(uploadedIDs, expectedID)
	}).Return(func(_ context.Context, folder, publicID string, _ interface{}) (*uploader.UploadResult, error) {
		return &uploader.UploadResult{
			PublicID: folder + "/" + publicID,
			URL:      "https://example.test/" + publicID + ".jpg",
		}, nil
	}).Times(len(imageHeaders))

	repo.On("InsertImageUrl", ctx, mock.MatchedBy(func(image *models.Images) bool {
		return image != nil && image.ProductId == product.ProductId
	})).Return(func(_ context.Context, image *models.Images) (models.Images, error) {
		return *image, nil
	}).Twice()

	response, err := u.CreateProduct(ctx, product, imageHeaders)
	require.NoError(t, err)
	require.NotNil(t, response)
	require.Len(t, response.Product.Images, 2)
	require.Len(t, stagedIDs, 3)
	require.Len(t, stagedIDs[0], 1)
	require.Equal(t, uploadedIDs[0], stagedIDs[0][0])
	require.Equal(t, uploadedIDs, stagedIDs[1])
	require.Equal(t, uploadedIDs[1:], stagedIDs[2])
}

func TestCreateProductReviewRejectsInvalidRating(t *testing.T) {
	cld := mockCloudinary.NewCloudUploader(t)
	repo := mockProd.NewRepo(t)
	u := usecase.NewProductsUC(cld, repo)

	err := u.CreateProductReview(context.Background(), models.Reviews{Rating: 6})
	require.ErrorIs(t, err, productdomain.ErrInvalidRating)
	repo.AssertNotCalled(t, "InsertReviewAndUpdateProduct", mock.Anything, mock.Anything)
}
