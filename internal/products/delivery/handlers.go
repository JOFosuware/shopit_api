// Package delivery provides HTTP handlers for product endpoints.
//
// It wires handler methods for creating, listing, updating, and deleting
// products, as well as for product reviews.
package delivery

import (
	"errors"
	"fmt"
	"math"
	"mime/multipart"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jofosuware/go/shopit/internal/models"
	"github.com/jofosuware/go/shopit/internal/products"
	"github.com/jofosuware/go/shopit/pkg/logger"
	"github.com/jofosuware/go/shopit/pkg/utils"
)

// UserContextKey is the request context key used to store the authenticated user.
const UserContextKey = utils.UserContextKey

// ProdHandlers provides HTTP handler methods for product endpoints.
type ProdHandlers struct {
	logger logger.Logger
	prodUC products.ProductUC
}

// NewProdHandlers returns a new ProdHandlers with the provided logger and usecase.
func NewProdHandlers(logger logger.Logger, prodUC products.ProductUC) *ProdHandlers {
	return &ProdHandlers{
		logger: logger,
		prodUC: prodUC,
	}
}

// CreateProduct creates a new product (admin).
// Endpoint: POST /api/v1/product/admin/product/new
// Expects form data: name, price, description, images, category, seller, stock.
func (h *ProdHandlers) CreateProduct(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(UserContextKey).(*models.User)
	if !ok || user == nil {
		_ = utils.InvalidCredentials(w)
		return
	}
	p, images, validationErrors, err := parseProductForm(r, user.ID)
	if err != nil {
		h.writeBadRequest(w, "parsing product form", err)
		return
	}
	if len(validationErrors) > 0 {
		h.writeValidationError(w, r, "validating product form", validationErrors)
		return
	}

	res, err := h.prodUC.CreateProduct(r.Context(), p, images)
	if err != nil {
		h.writeProductError(w, "creating product", err)
		return
	}

	_ = h.writeJSON(w, http.StatusOK, res)
}

// GetProducts returns a list of products.
// Endpoint: GET /api/v1/product/products
// Query params: keyword, page.
func (h *ProdHandlers) GetProducts(w http.ResponseWriter, r *http.Request) {
	keyword := r.URL.Query().Get("keyword")
	page := 1
	if pageParam := r.URL.Query().Get("page"); pageParam != "" {
		var err error
		page, err = strconv.Atoi(pageParam)
		if err != nil || !products.IsValidProductPage(page) {
			h.writeBadRequest(w, "parsing product page", products.ErrInvalidPage)
			return
		}
	}

	res, err := h.prodUC.GetProducts(r.Context(), keyword, page)
	if err != nil {
		h.writeProductError(w, "getting products", err)
		return
	}

	_ = h.writeJSON(w, http.StatusOK, res)
}

// GetAdminProducts returns all products (admin).
// Endpoint: GET /api/v1/product/admin/products
func (h *ProdHandlers) GetAdminProducts(w http.ResponseWriter, r *http.Request) {
	prods, err := h.prodUC.GetAdminProducts(r.Context())
	if err != nil {
		h.writeProductError(w, "getting admin products", err)
		return
	}

	jr := struct {
		Success  bool              `json:"success"`
		Products []*models.Product `json:"products"`
	}{
		Success:  true,
		Products: prods,
	}

	_ = h.writeJSON(w, http.StatusOK, jr)
}

// GetSingleProduct returns a product by ID.
// Endpoint: GET /api/v1/product/product/{id}
func (h *ProdHandlers) GetSingleProduct(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if id == "" {
		h.writeBadRequest(w, "parsing product ID", errors.New("id is empty"))
		return
	}

	parsedId, err := uuid.Parse(id)
	if err != nil {
		h.writeBadRequest(w, "parsing product ID", err)
		return
	}

	res, err := h.prodUC.GetSingleProduct(r.Context(), parsedId)
	if err != nil {
		h.writeProductError(w, "getting product", err)
		return
	}

	jr := models.ProdResponse{
		Success: true,
		Product: *res,
	}

	_ = h.writeJSON(w, http.StatusOK, jr)
}

// UpdateProduct updates a product (admin).
// Endpoint: PUT /api/v1/product/admin/product/{id}
// Expects form data similar to product creation.
func (h *ProdHandlers) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(UserContextKey).(*models.User)
	if !ok || user == nil {
		_ = utils.InvalidCredentials(w)
		return
	}

	id := chi.URLParam(r, "id")

	if id == "" {
		h.writeBadRequest(w, "parsing product ID", errors.New("id is empty"))
		return
	}
	parsedId, err := uuid.Parse(id)
	if err != nil {
		h.writeBadRequest(w, "parsing product ID", err)
		return
	}

	p, images, validationErrors, err := parseProductForm(r, user.ID)
	if err != nil {
		h.writeBadRequest(w, "parsing product form", err)
		return
	}
	if len(validationErrors) > 0 {
		h.writeValidationError(w, r, "validating product form", validationErrors)
		return
	}

	res, err := h.prodUC.UpdateProduct(r.Context(), parsedId, p, images)
	if err != nil {
		h.writeProductError(w, "updating product", err)
		return
	}

	_ = h.writeJSON(w, http.StatusOK, res)
}

func parseProductForm(r *http.Request, userID uuid.UUID) (models.Product, []*multipart.FileHeader, map[string]string, error) {
	if err := r.ParseMultipartForm(100000); err != nil {
		return models.Product{}, nil, nil, fmt.Errorf("parsing multipart form: %w", err)
	}

	name := r.Form.Get("name")
	description := r.Form.Get("description")
	seller := r.Form.Get("seller")
	validationErrors := make(map[string]string)
	if name == "" || description == "" || seller == "" {
		if name == "" {
			validationErrors["name"] = "product name must be provided"
		}
		if description == "" {
			validationErrors["description"] = "product description must be provided"
		}
		if seller == "" {
			validationErrors["seller"] = "product seller must be provided"
		}
	}

	price, err := strconv.ParseFloat(r.Form.Get("price"), 64)
	if err != nil || math.IsNaN(price) || math.IsInf(price, 0) || price <= 0 {
		validationErrors["price"] = "price must be a positive number"
	}
	stock, err := strconv.Atoi(r.Form.Get("stock"))
	if err != nil || stock < 0 {
		validationErrors["stock"] = "stock must be a non-negative integer"
	}

	var images []*multipart.FileHeader
	if r.MultipartForm != nil {
		images = r.MultipartForm.File["images"]
	}
	if len(validationErrors) > 0 {
		return models.Product{}, nil, validationErrors, nil
	}
	return models.Product{
		Name:        name,
		Price:       price,
		Description: description,
		Category:    r.Form.Get("category"),
		Seller:      seller,
		Stock:       stock,
		UserId:      userID,
	}, images, nil, nil
}

// DeleteProduct deletes a product (admin).
// Endpoint: DELETE /api/v1/product/admin/product/{id}
func (h *ProdHandlers) DeleteProduct(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if id == "" {
		h.writeBadRequest(w, "parsing product ID", errors.New("id is empty"))
		return
	}

	parsedId, err := uuid.Parse(id)
	if err != nil {
		h.writeBadRequest(w, "parsing product ID", err)
		return
	}

	err = h.prodUC.DeleteProduct(r.Context(), parsedId)
	if err != nil {
		h.writeProductError(w, "deleting product", err)
		return
	}

	jr := struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}{
		Success: true,
		Message: "product deleted successfully",
	}

	_ = h.writeJSON(w, http.StatusOK, jr)
}

// CreateProductReview creates a new review for a product.
// Endpoint: POST /api/v1/product/review
// Expects form data: rating, comment, productId.
func (h *ProdHandlers) CreateProductReview(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(UserContextKey).(*models.User)
	if !ok || user == nil {
		_ = utils.InvalidCredentials(w)
		return
	}

	err := r.ParseMultipartForm(100000)
	if err != nil {
		h.writeBadRequest(w, "parsing product review form", err)
		return
	}

	rating := r.Form.Get("rating")
	comment := r.Form.Get("comment")
	productId := r.Form.Get("productId")

	rtg, err := strconv.Atoi(rating)
	if err != nil || rtg < 1 || rtg > 5 {
		h.writeValidationError(w, r, "validating product review", map[string]string{
			"rating": products.ErrInvalidRating.Error(),
		})
		return
	}
	parsedProdId, err := uuid.Parse(productId)
	if err != nil {
		h.writeValidationError(w, r, "validating product review", map[string]string{
			"productId": "valid product ID must be provided",
		})
		return
	}

	review := models.Reviews{
		UserId:    user.ID,
		Name:      user.Name,
		Rating:    rtg,
		Comment:   comment,
		ProductId: parsedProdId,
	}

	err = h.prodUC.CreateProductReview(r.Context(), review)
	if err != nil {
		h.writeProductError(w, "creating product review", err)
		return
	}

	jr := struct {
		Success bool `json:"success"`
	}{
		Success: true,
	}

	_ = h.writeJSON(w, http.StatusOK, jr)
}

// GetProductReviews returns reviews for a product.
// Endpoint: GET /api/v1/product/reviews
// Query param: id (product ID).
func (h *ProdHandlers) GetProductReviews(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		h.writeBadRequest(w, "parsing product ID", errors.New("id is empty"))
		return
	}

	parsedId, err := uuid.Parse(id)
	if err != nil {
		h.writeBadRequest(w, "parsing product ID", err)
		return
	}

	reviews, err := h.prodUC.GetProductReviews(r.Context(), parsedId)
	if err != nil {
		h.writeProductError(w, "getting product reviews", err)
		return
	}

	jr := struct {
		Success bool             `json:"success"`
		Reviews []models.Reviews `json:"reviews"`
	}{
		Success: true,
		Reviews: reviews,
	}

	_ = h.writeJSON(w, http.StatusOK, jr)
}

// DeleteProductReview deletes a product review.
// Endpoint: DELETE /api/v1/product/reviews
// Query params: productId, id (review ID).
func (h *ProdHandlers) DeleteProductReview(w http.ResponseWriter, r *http.Request) {
	productId := r.URL.Query().Get("productId")
	reviewId := r.URL.Query().Get("id")
	if productId == "" || reviewId == "" {
		h.writeBadRequest(w, "reading product and review IDs", errors.New("IDs must be provided"))
		return
	}
	parsedProductId, err := uuid.Parse(productId)
	if err != nil {
		h.writeBadRequest(w, "parsing product ID", err)
		return
	}

	parsedId, err := uuid.Parse(reviewId)
	if err != nil {
		h.writeBadRequest(w, "parsing review ID", err)
		return
	}

	user, ok := r.Context().Value(UserContextKey).(*models.User)
	if !ok || user == nil {
		_ = utils.InvalidCredentials(w)
		return
	}
	err = h.prodUC.DeleteProductReview(r.Context(), user, parsedProductId, parsedId)
	if err != nil {
		h.writeProductError(w, "deleting product review", err)
		return
	}

	jr := struct {
		Success bool `json:"success"`
	}{
		Success: true,
	}

	_ = h.writeJSON(w, http.StatusOK, jr)
}
