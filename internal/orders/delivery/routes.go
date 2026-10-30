package delivery

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jofosuware/go/shopit/pkg/observability"
	"github.com/jofosuware/go/shopit/pkg/utils"
)

func (h *OrderHandlers) OrderRouter() http.Handler {
	mux := chi.NewRouter()

	mux.Use(utils.IsAuthenticated)

	mux.Post("/new", observability.Handler("orders.handler.create", h.CreateOrder))
	mux.Get("/{id}", observability.Handler("orders.handler.get", h.GetSingleOrder))
	mux.Get("/me", observability.Handler("orders.handler.list_user", h.GetUserOrders))
	mux.Group(func(admin chi.Router) {
		admin.Use(utils.AdminOnly)
		admin.Get("/admin/orders", observability.Handler("orders.handler.list", h.GetAllOrders))
		admin.Put("/admin/order/{id}", observability.Handler("orders.handler.update", h.UpdateOrder))
		admin.Delete("/admin/order/{id}", observability.Handler("orders.handler.delete", h.DeleteOrder))
	})

	return mux
}
