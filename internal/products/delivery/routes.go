package delivery

import (
	"net/http"

	"github.com/jofosuware/go/shopit/pkg/utils"

	"github.com/go-chi/chi/v5"
)

func (h *ProdHandlers) ProdRouter() http.Handler {
	mux := chi.NewRouter()

	mux.Get("/products", h.GetProducts)
	mux.Get("/product/{id}", h.GetSingleProduct)

	mux.Group(func(r chi.Router) {
		r.Use(utils.IsAuthenticated)

		r.Post("/new", h.CreateProduct)
		r.Get("/admin/products", h.GetAdminProducts)
		r.Put("/admin/product/{id}", h.UpdateProduct)
		r.Delete("/admin/product/{id}", h.DeleteProduct)
		r.Put("/review", h.CreateProductReview)
		r.Get("/reviews", h.GetProductReviews)
		r.Delete("/reviews", h.DeleteProductReview)
	})

	return mux
}
