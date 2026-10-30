package delivery

import (
	"net/http"

	"github.com/jofosuware/go/shopit/pkg/observability"
	"github.com/jofosuware/go/shopit/pkg/utils"

	"github.com/go-chi/chi/v5"
)

func (h *ProdHandlers) ProdRouter() http.Handler {
	mux := chi.NewRouter()

	mux.Get("/products", observability.Handler("products.handler.list", h.GetProducts))
	mux.Get("/product/{id}", observability.Handler("products.handler.get", h.GetSingleProduct))

	mux.Group(func(r chi.Router) {
		r.Use(utils.IsAuthenticated)

		r.Put("/review", observability.Handler("products.handler.create_review", h.CreateProductReview))
		r.Get("/reviews", observability.Handler("products.handler.list_reviews", h.GetProductReviews))
		r.Delete("/reviews", observability.Handler("products.handler.delete_review", h.DeleteProductReview))

		r.Group(func(r chi.Router) {
			r.Use(utils.AdminOnly)
			r.Post("/new", observability.Handler("products.handler.create", h.CreateProduct))
			r.Get("/admin/products", observability.Handler("products.handler.list_admin", h.GetAdminProducts))
			r.Put("/admin/product/{id}", observability.Handler("products.handler.update", h.UpdateProduct))
			r.Delete("/admin/product/{id}", observability.Handler("products.handler.delete", h.DeleteProduct))
		})
	})

	return mux
}
