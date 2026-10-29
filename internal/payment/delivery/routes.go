package delivery

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jofosuware/go/shopit/pkg/utils"
)

func (h *PaymentHandler) PaymentRouter() http.Handler {
	mux := chi.NewRouter()

	mux.Group(func(r chi.Router) {
		r.Use(utils.IsAuthenticated)

		r.Post("/process", h.ProcessPayment)
		r.Get("/stripeapi", h.SendStripeApi)
	})

	return mux
}
