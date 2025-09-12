package server

import (
	authHTTP "github.com/jofosuware/go/shopit/internal/auth/delivery"
	authRepository "github.com/jofosuware/go/shopit/internal/auth/repository"
	authUC "github.com/jofosuware/go/shopit/internal/auth/usecase"
	ordHTTP "github.com/jofosuware/go/shopit/internal/orders/delivery"
	ordRepository "github.com/jofosuware/go/shopit/internal/orders/repository"
	ordUC "github.com/jofosuware/go/shopit/internal/orders/usecase"
	payHTTP "github.com/jofosuware/go/shopit/internal/payment/delivery"
	prodHTTP "github.com/jofosuware/go/shopit/internal/products/delivery"
	prodRepository "github.com/jofosuware/go/shopit/internal/products/repository"
	prodUC "github.com/jofosuware/go/shopit/internal/products/usecase"
	"github.com/jofosuware/go/shopit/pkg/bcrypt"
	"github.com/jofosuware/go/shopit/pkg/card"
	"github.com/jofosuware/go/shopit/pkg/cloudinary"
	"github.com/jofosuware/go/shopit/pkg/mailer"
	"github.com/jofosuware/go/shopit/pkg/token"
	"github.com/jofosuware/go/shopit/pkg/utils"
)

// Setup instantiate handlers and repositories
func (s *Serve) Setup() {
	cld, err := cloudinary.NewCloudinary(s.cfg)
	if err != nil {
		s.logger.Fatal(err)
	}

	// Auth setups
	authRepo := authRepository.NewAuthRepository(s.DB)
	authUseCase := authUC.NewAuthUC(cld, authRepo, token.NewToken(), bcrypt.NewEncrypt(), mailer.NewMail(s.cfg))
	authHandlers = authHTTP.NewAuthHandlers(s.logger, authUseCase)

	// UTILS
	utils.Repo = authRepo

	// Product setups
	prodRepo := prodRepository.NewProdRepository(s.DB)
	prodUseCase := prodUC.NewProductsUC(cld, prodRepo)
	prodHandlers = prodHTTP.NewProdHandlers(s.logger, prodUseCase)

	// Order setups
	ordRepo := ordRepository.NewOrdersRepository(s.DB)
	ordUseCase := ordUC.NewOrderUC(ordRepo)
	ordHandlers = ordHTTP.NewOrderHandlers(s.logger, ordUseCase)

	// Payment setups
	cd := card.Card{
		Secret:   s.cfg.Stripe.Secret,
		Key:      s.cfg.Stripe.Key,
		Currency: "usd",
	}
	payHandlers = payHTTP.NewPaymentHandler(s.cfg, s.logger, &cd)
}
