package delivery_test

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jofosuware/go/shopit/internal/models"
	productdomain "github.com/jofosuware/go/shopit/internal/products"
	"github.com/jofosuware/go/shopit/internal/products/delivery"
	prodMock "github.com/jofosuware/go/shopit/internal/products/mocks"
	mockLogger "github.com/jofosuware/go/shopit/pkg/logger/mock"
	"github.com/jofosuware/go/shopit/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const UserContextKey = utils.UserContextKey

func TestAddProduct(t *testing.T) {
	logger := mockLogger.NewLogger(t)
	prodUC := prodMock.NewProductUC(t)

	h := delivery.NewProdHandlers(logger, prodUC)
	t.Run("Product added successfully", func(t *testing.T) {
		formData := url.Values{
			"name":        {"test"},
			"price":       {"100"},
			"description": {"test"},
			"category":    {"home"},
			"stock":       {"100"},
			"seller":      {"test"},
			"images":      {"something1.jpg", "something2.jpg"},
		}

		payload, ct, _ := utils.CreateMultipartForm(formData)
		req, err := http.NewRequest("POST", "/products", payload)
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		req.Header.Set("Content-Type", ct)

		err = req.ParseMultipartForm(100000)
		require.NoError(t, err)

		multipartForm := req.MultipartForm
		images := multipartForm.File["images"]
		price, _ := strconv.ParseFloat(formData.Get("price"), 64)
		stock, _ := strconv.Atoi(formData.Get("stock"))

		user := models.User{
			ID: uuid.New(),
		}

		// mock session
		ctx := context.WithValue(req.Context(), UserContextKey, &user)
		req = req.WithContext(ctx)

		prodUC.On("CreateProduct", req.Context(), models.Product{
			Name:        formData.Get("name"),
			Price:       price,
			Description: formData.Get("description"),
			Category:    formData.Get("category"),
			Stock:       stock,
			Seller:      formData.Get("seller"),
			UserId:      user.ID,
		}, images).Return(&models.ProdResponse{}, nil)

		h.CreateProduct(rr, req)

		got := rr.Code
		want := http.StatusOK

		assert.Equal(t, want, got)
	})

}

func TestGetProducts(t *testing.T) {
	logger := mockLogger.NewLogger(t)
	prodUC := prodMock.NewProductUC(t)

	h := delivery.NewProdHandlers(logger, prodUC)

	t.Run("Products retrieved successfully", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/products", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		prodUC.On("GetProducts", req.Context(), "", 1).Return(&models.GetProd{}, nil)

		h.GetProducts(rr, req)

		got := rr.Code
		want := http.StatusOK

		assert.Equal(t, want, got)
	})
}

func TestGetProductsRejectsInvalidPage(t *testing.T) {
	logger := mockLogger.NewLogger(t)
	logger.On("Errorf", "%s: %v", "parsing product page", productdomain.ErrInvalidPage).Return()
	prodUC := prodMock.NewProductUC(t)
	h := delivery.NewProdHandlers(logger, prodUC)

	req := httptest.NewRequest(http.MethodGet, "/products?page=not-a-number", nil)
	rr := httptest.NewRecorder()
	h.GetProducts(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	prodUC.AssertNotCalled(t, "GetProducts", mock.Anything, mock.Anything)
}

func TestCreateProductRejectsInvalidPrice(t *testing.T) {
	logger := mockLogger.NewLogger(t)
	logger.On("Errorf", "%s: %v", "validating product form", mock.Anything).Return()
	prodUC := prodMock.NewProductUC(t)
	h := delivery.NewProdHandlers(logger, prodUC)

	fields := url.Values{
		"name":        {"test"},
		"price":       {"NaN"},
		"description": {"test"},
		"stock":       {"1"},
		"seller":      {"test"},
	}
	payload, contentType, err := utils.CreateMultipartForm(fields)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/products", payload)
	req.Header.Set("Content-Type", contentType)
	req = req.WithContext(context.WithValue(req.Context(), UserContextKey, &models.User{ID: uuid.New()}))
	rr := httptest.NewRecorder()

	h.CreateProduct(rr, req)

	assert.Equal(t, http.StatusUnprocessableEntity, rr.Code)
	assert.Contains(t, rr.Body.String(), `"price": "price must be a positive number"`)
	prodUC.AssertNotCalled(t, "CreateProduct", mock.Anything, mock.Anything)
}

func TestCreateProductMapsImageUploadFailureToBadGateway(t *testing.T) {
	logger := mockLogger.NewLogger(t)
	logger.On("Errorf", "%s: %v", "creating product", mock.Anything).Return()
	prodUC := prodMock.NewProductUC(t)
	h := delivery.NewProdHandlers(logger, prodUC)

	fields := url.Values{
		"name":        {"test"},
		"price":       {"10"},
		"description": {"test"},
		"stock":       {"1"},
		"seller":      {"test"},
	}
	payload, contentType, err := utils.CreateMultipartForm(fields)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/products", payload)
	req.Header.Set("Content-Type", contentType)
	req = req.WithContext(context.WithValue(req.Context(), UserContextKey, &models.User{ID: uuid.New()}))
	prodUC.On("CreateProduct", mock.Anything, mock.Anything, mock.Anything).
		Return((*models.ProdResponse)(nil), fmt.Errorf("%w: provider details", productdomain.ErrImageUploadFailed))
	rr := httptest.NewRecorder()

	h.CreateProduct(rr, req)

	assert.Equal(t, http.StatusBadGateway, rr.Code)
	assert.Contains(t, rr.Body.String(), productdomain.ErrImageUploadFailed.Error())
	assert.NotContains(t, rr.Body.String(), "provider details")
}

func TestGetAdminProducts(t *testing.T) {
	logger := mockLogger.NewLogger(t)
	prodUC := prodMock.NewProductUC(t)

	h := delivery.NewProdHandlers(logger, prodUC)

	t.Run("Products retrieved successfully", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/admin/products", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		prodUC.On("GetAdminProducts", req.Context()).Return([]*models.Product{}, nil)

		h.GetAdminProducts(rr, req)

		got := rr.Code
		want := http.StatusOK

		assert.Equal(t, want, got)
	})
}

func TestGetSingleProduct(t *testing.T) {
	logger := mockLogger.NewLogger(t)
	prodUC := prodMock.NewProductUC(t)

	h := delivery.NewProdHandlers(logger, prodUC)

	t.Run("Product retrieved successfully", func(t *testing.T) {
		id := uuid.New()

		req, err := http.NewRequest("GET", "/product/"+id.String(), nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		// Create a chi router and set the URL param
		rCtx := chi.NewRouteContext()
		rCtx.URLParams.Add("id", id.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rCtx))

		prodUC.On("GetSingleProduct", req.Context(), id).Return(&models.Product{}, nil)

		h.GetSingleProduct(rr, req)

		got := rr.Code
		want := http.StatusOK

		assert.Equal(t, want, got)
	})
}

func TestGetSingleProductReturnsNotFound(t *testing.T) {
	logger := mockLogger.NewLogger(t)
	logger.On("Errorf", "%s: %v", "getting product", mock.Anything).Return()
	prodUC := prodMock.NewProductUC(t)
	h := delivery.NewProdHandlers(logger, prodUC)
	id := uuid.New()

	req := httptest.NewRequest(http.MethodGet, "/product/"+id.String(), nil)
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("id", id.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeContext))
	prodUC.On("GetSingleProduct", req.Context(), id).Return((*models.Product)(nil), fmt.Errorf("%w: %w", productdomain.ErrProductNotFound, sql.ErrNoRows))
	rr := httptest.NewRecorder()

	h.GetSingleProduct(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
	assert.Contains(t, rr.Body.String(), `"message": "product not found"`)
	assert.NotContains(t, rr.Body.String(), sql.ErrNoRows.Error())
}

func TestUpdateProduct(t *testing.T) {
	logger := mockLogger.NewLogger(t)
	prodUC := prodMock.NewProductUC(t)

	h := delivery.NewProdHandlers(logger, prodUC)

	t.Run("Product updated successfully", func(t *testing.T) {
		id := uuid.New()

		formData := url.Values{
			"name":        {"test"},
			"price":       {"100"},
			"description": {"test"},
			"category":    {"test"},
			"stock":       {"100"},
			"seller":      {"test"},
			"images":      {"something1.jpg", "something2.jpg"},
		}

		payload, ct, _ := utils.CreateMultipartForm(formData)

		req, err := http.NewRequest("PUT", "/product/id", payload)
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		req.Header.Set("Content-Type", ct)

		// Create a chi router and set the URL param
		rCtx := chi.NewRouteContext()
		rCtx.URLParams.Add("id", id.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rCtx))

		err = req.ParseMultipartForm(100000)
		require.NoError(t, err)

		multipartForm := req.MultipartForm
		images := multipartForm.File["images"]
		price, _ := strconv.ParseFloat(formData.Get("price"), 64)
		stock, _ := strconv.Atoi(formData.Get("stock"))

		user := models.User{
			ID: uuid.New(),
		}

		// mock session
		ctx := context.WithValue(req.Context(), UserContextKey, &user)
		req = req.WithContext(ctx)

		prodUC.On("UpdateProduct", req.Context(), id, models.Product{
			Name:        formData.Get("name"),
			Price:       price,
			Description: formData.Get("description"),
			Category:    formData.Get("category"),
			Stock:       stock,
			Seller:      formData.Get("seller"),
			UserId:      user.ID,
		}, images).Return(&models.ProdResponse{}, nil)

		h.UpdateProduct(rr, req)

		got := rr.Code
		want := http.StatusOK

		assert.Equal(t, want, got)
	})
}

func TestDeleteProduct(t *testing.T) {
	logger := mockLogger.NewLogger(t)
	prodUC := prodMock.NewProductUC(t)

	h := delivery.NewProdHandlers(logger, prodUC)

	t.Run("Product deleted successfully", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodDelete, "/product/id", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		id := uuid.New()

		// Create a chi router and set the URL param
		rCtx := chi.NewRouteContext()
		rCtx.URLParams.Add("id", id.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rCtx))

		prodUC.On("DeleteProduct", req.Context(), id).Return(nil)

		h.DeleteProduct(rr, req)

		got := rr.Code
		want := http.StatusOK

		assert.Equal(t, want, got)
	})
}

func TestCreateProductReview(t *testing.T) {
	logger := mockLogger.NewLogger(t)
	prodUC := prodMock.NewProductUC(t)

	h := delivery.NewProdHandlers(logger, prodUC)

	t.Run("Product review created successfully", func(t *testing.T) {

		formData := url.Values{
			"rating":    {"5"},
			"comment":   {"test"},
			"productId": {uuid.NewString()},
		}

		payload, ct, _ := utils.CreateMultipartForm(formData)

		req, err := http.NewRequest(http.MethodPost, "/product/id/review", payload)
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		req.Header.Set("Content-Type", ct)

		_ = req.ParseMultipartForm(100000)
		rating, _ := strconv.Atoi(req.Form.Get("rating"))
		comment := req.Form.Get("comment")
		productId := req.Form.Get("productId")
		userId := uuid.New()

		parsedProdId, _ := uuid.Parse(productId)
		review := models.Reviews{
			Rating:    rating,
			Comment:   comment,
			UserId:    userId,
			ProductId: parsedProdId,
		}

		user := models.User{
			ID: userId,
		}

		// mock session
		ctx := context.WithValue(req.Context(), UserContextKey, &user)
		req = req.WithContext(ctx)

		prodUC.On("CreateProductReview", req.Context(), review).Return(nil)

		h.CreateProductReview(rr, req)

		got := rr.Code
		want := http.StatusOK

		assert.Equal(t, want, got)
	})
}

func TestGetProductReviews(t *testing.T) {
	logger := mockLogger.NewLogger(t)
	prodUC := prodMock.NewProductUC(t)

	h := delivery.NewProdHandlers(logger, prodUC)

	t.Run("Product reviews fetched successfully", func(t *testing.T) {
		id := uuid.New()

		req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("/product/reviews?id=%v", id), nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		prodUC.On("GetProductReviews", req.Context(), id).Return([]models.Reviews{}, nil)

		h.GetProductReviews(rr, req)

		got := rr.Code
		want := http.StatusOK

		assert.Equal(t, want, got)
	})
}

func TestDeleteProductReview(t *testing.T) {
	logger := mockLogger.NewLogger(t)
	prodUC := prodMock.NewProductUC(t)

	h := delivery.NewProdHandlers(logger, prodUC)

	t.Run("Product review deleted successfully", func(t *testing.T) {
		prodId := uuid.New()
		rId := uuid.New()
		req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("/product/reviews/?id=%s&productId=%s", rId, prodId), nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		productId := req.URL.Query().Get("productId")
		reviewId := req.URL.Query().Get("id")

		parsedProdId, _ := uuid.Parse(productId)
		parsedReviewId, _ := uuid.Parse(reviewId)
		user := &models.User{ID: uuid.New()}
		req = req.WithContext(context.WithValue(req.Context(), UserContextKey, user))

		prodUC.On("DeleteProductReview", req.Context(), user, parsedProdId, parsedReviewId).Return(nil)

		h.DeleteProductReview(rr, req)

		got := rr.Code
		want := http.StatusOK

		assert.Equal(t, want, got)
	})
}
