package server

import (
	"strings"
	"time"

	authHTTP "github.com/jofosuware/go/shopit/internal/auth/delivery"
	authRepository "github.com/jofosuware/go/shopit/internal/auth/repository"
	authUC "github.com/jofosuware/go/shopit/internal/auth/usecase"
	ordHTTP "github.com/jofosuware/go/shopit/internal/orders/delivery"
	ordRepository "github.com/jofosuware/go/shopit/internal/orders/repository"
	ordUC "github.com/jofosuware/go/shopit/internal/orders/usecase"
	payHTTP "github.com/jofosuware/go/shopit/internal/payment/delivery"
	productCleanup "github.com/jofosuware/go/shopit/internal/products/cleanup"
	prodHTTP "github.com/jofosuware/go/shopit/internal/products/delivery"
	prodRepository "github.com/jofosuware/go/shopit/internal/products/repository"
	prodUC "github.com/jofosuware/go/shopit/internal/products/usecase"
	"github.com/jofosuware/go/shopit/pkg/bcrypt"
	"github.com/jofosuware/go/shopit/pkg/card"
	"github.com/jofosuware/go/shopit/pkg/cloudinary"
	"github.com/jofosuware/go/shopit/pkg/mailer"
	"github.com/jofosuware/go/shopit/pkg/ratelimiter"
	"github.com/jofosuware/go/shopit/pkg/token"
	"github.com/jofosuware/go/shopit/pkg/utils"
	"golang.org/x/time/rate"
)

// Setup instantiate handlers and repositories
func (s *Serve) Setup() {
	trustedProxyCIDRs := splitTrustedProxyCIDRs(s.cfg.Server.RateLimitTrustedProxies)
	var err error
	s.apiRateLimiter, err = ratelimiter.NewRateLimiterWithOptions(ratelimiter.Options{
		Rate:              rate.Every(200 * time.Millisecond),
		Burst:             100,
		MaxVisitors:       10_000,
		VisitorTTL:        10 * time.Minute,
		TrustedProxyCIDRs: trustedProxyCIDRs,
	})
	if err != nil {
		s.logger.Fatalf("configuring API rate limiter: %v", err)
		return
	}

	cld, err := cloudinary.NewCloudinary(s.cfg)
	if err != nil {
		s.logger.Fatal(err)
	}

	// Auth setups
	authRepo := authRepository.NewAuthRepository(s.DB)
	authUseCase := authUC.NewAuthUC(cld, authRepo, token.NewToken(), bcrypt.NewEncrypt(), mailer.NewMail(s.cfg), s.cfg.Frontend, s.cfg.EmailFrom)
	authHandlers, err = authHTTP.NewAuthHandlersWithTrustedProxies(s.logger, authUseCase, trustedProxyCIDRs)
	if err != nil {
		s.logger.Fatalf("configuring authentication rate limits: %v", err)
		return
	}

	// UTILS
	utils.Repo = authRepo

	// Product setups
	prodRepo := prodRepository.NewProdRepository(s.DB)
	s.productCleanupWorker = productCleanup.NewWorker(prodRepo, cld, s.logger, 30*time.Second)
	prodUseCase := prodUC.NewProductsUC(cld, prodRepo)
	prodHandlers = prodHTTP.NewProdHandlers(s.logger, prodUseCase)

	// Payment and order setups
	cd := card.Card{
		Secret:   s.cfg.Stripe.Secret,
		Key:      s.cfg.Stripe.Key,
		Currency: "usd",
	}

	ordRepo := ordRepository.NewOrdersRepository(s.DB)
	ordUseCase := ordUC.NewOrderUC(ordRepo, &cd)
	ordHandlers = ordHTTP.NewOrderHandlers(s.logger, ordUseCase)
	payHandlers = payHTTP.NewPaymentHandler(s.cfg, s.logger, ordUseCase)
}

func splitTrustedProxyCIDRs(configured string) []string {
	if strings.TrimSpace(configured) == "" {
		return nil
	}
	parts := strings.Split(configured, ",")
	cidrs := make([]string, 0, len(parts))
	for _, part := range parts {
		if cidr := strings.TrimSpace(part); cidr != "" {
			cidrs = append(cidrs, cidr)
		}
	}
	return cidrs
}
