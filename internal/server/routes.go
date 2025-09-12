package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

func (s *Serve) Routes() http.Handler {
	mux := chi.NewRouter()

	mux.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://shopit-1-87gz.onrender.com", "http://localhost:3000"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "Origin"},
		ExposedHeaders:   []string{"Link", "Access-Control-Allow-Credentials"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	mux.Use(middleware.Recoverer)

	mux.Mount("/api/v1/auth", authHandlers.AuthRouter())
	mux.Mount("/api/v1/product", prodHandlers.ProdRouter())
	mux.Mount("/api/v1/orders", ordHandlers.OrderRouter())
	mux.Mount("/api/v1/payment", payHandlers.PaymentRouter())

	return mux
}
