package delivery

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jofosuware/go/shopit/pkg/utils"
)

func (h *OrderHandlers) OrderRouter() http.Handler {
	mux := chi.NewRouter()

	mux.Use(utils.IsAuthenticated)

	mux.Post("/new", h.CreateOrder)
	mux.Get("/{id}", h.GetSingleOrder)
	mux.Get("/me", h.GetUserOrders)
	mux.Group(func(admin chi.Router) {
		admin.Use(utils.AdminOnly)
		admin.Get("/admin/orders", h.GetAllOrders)
		admin.Put("/admin/order/{id}", h.UpdateOrder)
		admin.Delete("/admin/order/{id}", h.DeleteOrder)
	})

	return mux
}
