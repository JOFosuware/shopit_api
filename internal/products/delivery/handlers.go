package delivery

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jofosuware/go/shopit/internal/models"
	"github.com/jofosuware/go/shopit/internal/products"
	"github.com/jofosuware/go/shopit/pkg/logger"
	"github.com/jofosuware/go/shopit/pkg/utils"
	"github.com/jofosuware/go/shopit/pkg/validator"
)

const UserContextKey = utils.UserContextKey

// ProdHandlers is Product handlers type
type ProdHandlers struct {
	logger logger.Logger
	prodUC products.ProductUC
}

// NewProdHandlers is the constructor for ProdHandlers
func NewProdHandlers(logger logger.Logger, prodUC products.ProductUC) *ProdHandlers {
	return &ProdHandlers{
		logger: logger,
		prodUC: prodUC,
	}
}

// CreateProduct create new product   =>   /api/v1/product/admin/product/new
func (h *ProdHandlers) CreateProduct(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(UserContextKey).(*models.User)
	if !ok {
		_ = utils.BadRequest(w, r, errors.New("user must login as admin to perform this task"))
		h.logger.Errorf("reading json error: %s", "user must login as admin to perform this task")
		return
	}
	var p models.Product

	// Parse form
	err := r.ParseMultipartForm(100000)
	if err != nil {
		_ = utils.BadRequest(w, r, errors.New("something went wrong, try again"))
		h.logger.Errorf("reading json error: %v", err)
		return
	}

	name := r.Form.Get("name")
	price, _ := strconv.ParseFloat(r.Form.Get("price"), 64)
	description := r.Form.Get("description")
	ratings, _ := strconv.Atoi(r.Form.Get("ratings"))
	multipartForm := r.MultipartForm
	images := multipartForm.File["images"]
	category := r.Form.Get("category")
	seller := r.Form.Get("seller")
	stock, _ := strconv.Atoi(r.Form.Get("stock"))

	// validate data
	v := validator.New()

	v.Check(name != "", "name", "product name must be provided")
	v.Check(description != "", "description", "product description must be provided")
	v.Check(seller != "", "seller", "product seller must be provided")

	if !v.Valid() {
		utils.FailedValidation(w, r, v.Errors)
		h.logger.Errorf("Failed validation: %v", v.Errors)
		return
	}

	p.Name = name
	p.Price = price
	p.Description = description
	p.Category = category
	p.Ratings = ratings
	p.Seller = seller
	p.Stock = stock
	p.UserId = user.ID

	res, err := h.prodUC.CreateProduct(p, images)
	if err != nil {
		_ = utils.BadRequest(w, r, errors.New("something went wrong, try again"))
		h.logger.Errorf("error creating product: %v", err)
		return
	}

	if err = utils.WriteJSON(w, http.StatusOK, res); err != nil {
		_ = utils.BadRequest(w, r, err)
		h.logger.Errorf("error writing json: %v", err)
		return
	}
}

// GetProducts get all products   =>   /api/v1/product/products?keyword=apple
func (h *ProdHandlers) GetProducts(w http.ResponseWriter, r *http.Request) {
	keyword := r.URL.Query().Get("keyword")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))

	res, err := h.prodUC.GetProducts(keyword, page)
	if err != nil {
		_ = utils.BadRequest(w, r, errors.New("something went wrong, try again"))
		h.logger.Errorf("error getting products: %v", err)
		return
	}

	if err = utils.WriteJSON(w, http.StatusOK, res); err != nil {
		_ = utils.BadRequest(w, r, err)
		h.logger.Errorf("error writing json: %v", err)
		return
	}
}

// GetAdminProducts get all products (Admin)  =>   /api/v1/product/admin/products
func (h *ProdHandlers) GetAdminProducts(w http.ResponseWriter, r *http.Request) {
	prods, err := h.prodUC.GetAdminProducts()
	if err != nil {
		_ = utils.BadRequest(w, r, errors.New("something went wrong, try again"))
		h.logger.Errorf("error getting products: %v", err)
		return
	}

	jr := struct {
		Success  bool              `json:"success"`
		Products []*models.Product `json:"products"`
	}{
		Success:  true,
		Products: prods,
	}

	if err = utils.WriteJSON(w, http.StatusOK, jr); err != nil {
		_ = utils.BadRequest(w, r, err)
		h.logger.Errorf("error writing json: %v", err)
		return
	}
}

// GetSingleProduct get single product details   =>   /api/v1/product/product/:id
func (h *ProdHandlers) GetSingleProduct(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if id == "" {
		_ = utils.BadRequest(w, r, errors.New("something went wrong, try again"))
		h.logger.Errorf("error parsing uuid: %v", errors.New("id is empty"))
		return
	}

	parsedId, err := uuid.Parse(id)
	if err != nil {
		_ = utils.BadRequest(w, r, errors.New("something went wrong, try again"))
		h.logger.Errorf("error parsing uuid: %v", err)
		return
	}

	res, err := h.prodUC.GetSingleProduct(parsedId)
	if err != nil {
		_ = utils.BadRequest(w, r, errors.New("something went wrong, try again"))
		h.logger.Errorf("error getting product: %v", err)
		return
	}

	jr := models.ProdResponse{
		Success: true,
		Product: *res,
	}

	if err = utils.WriteJSON(w, http.StatusOK, jr); err != nil {
		_ = utils.BadRequest(w, r, err)
		h.logger.Errorf("error writing json: %v", err)
		return
	}
}

// UpdateProduct update product   =>   /api/v1/product/admin/product/:id
func (h *ProdHandlers) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(UserContextKey).(*models.User)
	if !ok {
		_ = utils.BadRequest(w, r, errors.New("user must login as admin to perform this task"))
		h.logger.Errorf("reading json error: %s", "user must login as admin to perform this task")
		return
	}

	id := chi.URLParam(r, "id")

	if id == "" {
		_ = utils.BadRequest(w, r, errors.New("something went wrong, try again"))
		h.logger.Errorf("error parsing uuid: %v", errors.New("id is empty"))
		return
	}
	parsedId, err := uuid.Parse(id)
	if err != nil {
		_ = utils.BadRequest(w, r, errors.New("something went wrong, try again"))
		h.logger.Errorf("error parsing uuid: %v", err)
		return
	}

	var p models.Product

	// Parse form
	err = r.ParseMultipartForm(100000)
	if err != nil {
		_ = utils.BadRequest(w, r, errors.New("something went wrong, try again"))
		h.logger.Errorf("reading json error: %v", err)
		return
	}

	name := r.Form.Get("name")
	price, _ := strconv.ParseFloat(r.Form.Get("price"), 64)
	description := r.Form.Get("description")
	ratings, _ := strconv.Atoi(r.Form.Get("ratings"))
	multipartForm := r.MultipartForm
	images := multipartForm.File["images"]
	img, _ := utils.ExtractImages(images)
	category := r.Form.Get("category")
	seller := r.Form.Get("seller")
	stock, _ := strconv.Atoi(r.Form.Get("stock"))

	// validate data
	v := validator.New()

	v.Check(name != "", "name", "product name must be provided")
	v.Check(description != "", "description", "product description must be provided")
	v.Check(seller != "", "seller", "product seller must be provided")

	if !v.Valid() {
		utils.FailedValidation(w, r, v.Errors)
		h.logger.Errorf("Failed validation: %v", v.Errors)
		return
	}

	p.Name = name
	p.Price = price
	p.Description = description
	p.Category = category
	p.Ratings = ratings
	p.Seller = seller
	p.Stock = stock
	p.UserId = user.ID

	res, err := h.prodUC.UpdateProduct(parsedId, p, img)
	if err != nil {
		_ = utils.BadRequest(w, r, errors.New("something went wrong, try again"))
		h.logger.Errorf("error updating product: %v", err)
		return
	}

	if err = utils.WriteJSON(w, http.StatusOK, res); err != nil {
		_ = utils.BadRequest(w, r, err)
		h.logger.Errorf("error writing json: %v", err)
		return
	}
}

// DeleteProduct delete Product   =>   /api/v1/product/admin/product/:id
func (h *ProdHandlers) DeleteProduct(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if id == "" {
		_ = utils.BadRequest(w, r, errors.New("something went wrong, try again"))
		h.logger.Errorf("error parsing uuid: %v", errors.New("id is empty"))
		return
	}

	parsedId, err := uuid.Parse(id)
	if err != nil {
		_ = utils.BadRequest(w, r, errors.New("something went wrong, try again"))
		h.logger.Errorf("error parsing uuid: %v", err)
		return
	}

	err = h.prodUC.DeleteProduct(parsedId)
	if err != nil {
		_ = utils.BadRequest(w, r, errors.New("something went wrong, try again"))
		h.logger.Errorf("error deleting product: %v", err)
		return
	}

	jr := struct {
		success bool
		message string
	}{
		success: true,
		message: "product deleted successfully",
	}

	if err = utils.WriteJSON(w, http.StatusOK, jr); err != nil {
		_ = utils.BadRequest(w, r, err)
		h.logger.Errorf("error writing json: %v", err)
		return
	}
}

// CreateProductReview create new review   =>   /api/v1/product/review
func (h *ProdHandlers) CreateProductReview(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(UserContextKey).(*models.User)
	if !ok {
		_ = utils.BadRequest(w, r, errors.New("user cannot be found, login"))
		h.logger.Errorf("error getting user: %v", errors.New("user not found"))
		return
	}

	err := r.ParseMultipartForm(100000)
	if err != nil {
		_ = utils.BadRequest(w, r, errors.New("something went wrong, try again"))
		h.logger.Errorf("error parsing form: %v", err)
		return
	}

	rating := r.Form.Get("rating")
	comment := r.Form.Get("comment")
	productId := r.Form.Get("productId")

	rtg, _ := strconv.Atoi(rating)
	parsedProdId, _ := uuid.Parse(productId)

	review := models.Reviews{
		UserId:    user.ID,
		Name:      user.Name,
		Rating:    rtg,
		Comment:   comment,
		ProductId: parsedProdId,
	}

	err = h.prodUC.CreateProductReview(review)
	if err != nil {
		_ = utils.BadRequest(w, r, errors.New("something went wrong, try again"))
		h.logger.Errorf("error creating product review: %v", err)
		return
	}

	jr := struct {
		success bool
	}{
		success: true,
	}

	if err = utils.WriteJSON(w, http.StatusOK, jr); err != nil {
		_ = utils.BadRequest(w, r, err)
		h.logger.Errorf("error writing json: %v", err)
		return
	}
}

// GetProductReviews get Product Reviews   =>   /api/v1/product/reviews
func (h *ProdHandlers) GetProductReviews(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		_ = utils.BadRequest(w, r, errors.New("something went wrong, try again"))
		h.logger.Errorf("error parsing uuid: %v", errors.New("id is empty"))
		return
	}

	parsedId, err := uuid.Parse(id)
	if err != nil {
		_ = utils.BadRequest(w, r, errors.New("something went wrong, try again"))
		h.logger.Errorf("error parsing uuid: %v", err)
		return
	}

	reviews, err := h.prodUC.GetProductReviews(parsedId)
	if err != nil {
		_ = utils.BadRequest(w, r, errors.New("something went wrong, try again"))
		h.logger.Errorf("error getting product reviews: %v", err)
		return
	}

	jr := struct {
		Success bool             `json:"success"`
		Reviews []models.Reviews `json:"reviews"`
	}{
		Success: true,
		Reviews: reviews,
	}

	if err = utils.WriteJSON(w, http.StatusOK, jr); err != nil {
		_ = utils.BadRequest(w, r, err)
		h.logger.Errorf("error writing json: %v", err)
		return
	}
}

// DeleteProductReview delete Product Review   =>   /api/v1/product/reviews
func (h *ProdHandlers) DeleteProductReview(w http.ResponseWriter, r *http.Request) {
	productId := r.URL.Query().Get("productId")
	reviewId := r.URL.Query().Get("id")
	if productId == "" || reviewId == "" {
		_ = utils.BadRequest(w, r, errors.New("something went wrong, try again"))
		h.logger.Errorf("error retrieving ids: %v", errors.New("id is empty"))
		return
	}
	parsedProductId, err := uuid.Parse(productId)
	if err != nil {
		_ = utils.BadRequest(w, r, errors.New("something went wrong, try again"))
		h.logger.Errorf("error parsing uuid: %v", err)
		return
	}

	parsedId, err := uuid.Parse(reviewId)
	if err != nil {
		_ = utils.BadRequest(w, r, errors.New("something went wrong, try again"))
		h.logger.Errorf("error parsing uuid: %v", err)
		return
	}

	err = h.prodUC.DeleteProductReview(parsedProductId, parsedId)
	if err != nil {
		_ = utils.BadRequest(w, r, errors.New("something went wrong, try again"))
		h.logger.Errorf("error deleting product review: %v", err)
		return
	}

	jr := struct {
		Success bool `json:"success"`
	}{
		Success: true,
	}

	if err = utils.WriteJSON(w, http.StatusOK, jr); err != nil {
		_ = utils.BadRequest(w, r, err)
		h.logger.Errorf("error writing json: %v", err)
		return
	}
}
