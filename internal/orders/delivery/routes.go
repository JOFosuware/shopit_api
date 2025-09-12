package delivery

import (
	"github.com/go-chi/chi/v5"
	"github.com/jofosuware/go/shopit/pkg/utils"
	"net/http"
)

func (h *OrderHandlers) OrderRouter() http.Handler {
	mux := chi.NewRouter()

	mux.Use(utils.IsAuthenticated)

	mux.Post("/new", h.CreateOrder)
	mux.Get("/{id}", h.GetSingleOrder)
	mux.Get("/me", h.GetUserOrders)
	mux.Get("/admin/orders", h.GetAllOrders)
	mux.Put("/admin/order/{id}", h.UpdateOrder)
	mux.Delete("/admin/order/{id}", h.DeleteOrder)

	return mux
}
