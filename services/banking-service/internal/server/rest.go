package server

import (
	"banking-service/internal/config"
	"banking-service/internal/handler"
	"banking-service/internal/validator"
	"common/pkg/auth"
	"common/pkg/errors"
	"common/pkg/logging"
	"context"
	stderrors "errors"
	"log"
	"net/http"
	"time"

	_ "banking-service/docs"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.uber.org/fx"
)

func NewServer(
	lc fx.Lifecycle,
	cfg *config.Configuration,
	healthHandler *handler.HealthHandler,
	accountHandler *handler.AccountHandler,
	companyHandler *handler.CompanyHandler,
	payeeHandler *handler.PayeeHandler,
	exchangeHandler *handler.ExchangeHandler,
	paymentHandler *handler.PaymentHandler,
	cardHandler *handler.CardHandler,
	loanHandler *handler.LoanHandler,
	verifier auth.TokenVerifier,
	permissions auth.PermissionProvider,
) {
	r := gin.New()

	InitRouter(r, cfg)
	SetupRoutes(r, healthHandler, accountHandler, companyHandler, payeeHandler, exchangeHandler, paymentHandler,  cardHandler, loanHandler, verifier, permissions)

	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	RegisterServerLifecycle(lc, server)
}

func InitRouter(r *gin.Engine, cfg *config.Configuration) {
	r.Use(gin.Recovery())

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{cfg.URLs.FrontendBaseURL},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	r.Use(logging.Logger())
	r.Use(errors.ErrorHandler())

	validator.RegisterValidators()
}

func SetupRoutes(
	r *gin.Engine,
	healthHandler *handler.HealthHandler,
	accountHandler *handler.AccountHandler,
	companyHandler *handler.CompanyHandler,
	payeeHandler *handler.PayeeHandler,
	exchangeHandler *handler.ExchangeHandler,
	paymentHandler *handler.PaymentHandler,
	cardHandler *handler.CardHandler,
	loanHandler *handler.LoanHandler,
	verifier auth.TokenVerifier,
	permissions auth.PermissionProvider,
) {
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	api := r.Group("/api")
	{
		api.GET("/health", healthHandler.Health)

		accounts := api.Group("/accounts")
		accounts.Use(auth.Middleware(verifier, permissions))
		{
			accounts.POST("", accountHandler.Create)
      accounts.GET("/:accountId/cards", auth.RequireIdentityType(auth.IdentityClient, auth.IdentityEmployee), cardHandler.ListCardsByAccount)
			//TODO employee list all accounts here?
		}

		client := api.Group("/clients/:clientId")
		client.Use(auth.Middleware(verifier, permissions))
		{
			clientAccounts := client.Group("/accounts")
			{
				clientAccounts.GET("", accountHandler.GetClientAccounts)
				clientAccounts.GET("/:accountNumber", accountHandler.GetAccountDetails)
				clientAccounts.GET("/:accountNumber/payments", paymentHandler.GetAccountPayments)
				clientAccounts.PUT("/:accountNumber/name", accountHandler.UpdateAccountName)
				clientAccounts.POST("/:accountNumber/limits/request", accountHandler.RequestLimitsChange)
				clientAccounts.PUT("/:accountNumber/limits", accountHandler.ConfirmLimitsChange)
			}
			clientPayments := client.Group("/payments")
			{
				clientPayments.GET("", paymentHandler.GetClientPayments)
				clientPayments.GET("/:id", paymentHandler.GetPaymentByID)
				clientPayments.GET("/:id/receipt", paymentHandler.GetReceipt)
				clientPayments.POST("", paymentHandler.CreatePayment)
				clientPayments.POST("/:id/verify", paymentHandler.VerifyPayment)
			}
		}

		companies := api.Group("/companies")
		companies.Use(auth.Middleware(verifier, permissions))
		{
			companies.POST("", companyHandler.Create)
		}

		payees := api.Group("/payees")
		payees.Use(auth.Middleware(verifier, permissions))
		{
			payees.GET("", payeeHandler.GetAll)
			payees.POST("", payeeHandler.Create)
			payees.PATCH("/:id", payeeHandler.Update)
			payees.DELETE("/:id", payeeHandler.Delete)
		}

		cards := api.Group("/cards")
		cards.Use(auth.Middleware(verifier, permissions))
		{
			cards.POST("/request", auth.RequireIdentityType(auth.IdentityClient), cardHandler.RequestCard)
			cards.POST("/request/confirm", auth.RequireIdentityType(auth.IdentityClient), cardHandler.ConfirmCardRequest)
			cards.PUT("/:cardId/block", auth.RequireIdentityType(auth.IdentityClient, auth.IdentityEmployee), cardHandler.BlockCard)
			cards.PUT("/:cardId/unblock", auth.RequireIdentityType(auth.IdentityEmployee), cardHandler.UnblockCard)
			cards.PUT("/:cardId/deactivate", auth.RequireIdentityType(auth.IdentityEmployee), cardHandler.DeactivateCard)
    }
    
		exchange := api.Group("/exchange")
		{
			exchange.GET("/rates", exchangeHandler.GetRates)
			exchange.GET("/calculate", exchangeHandler.Calculate)
		}


		clientLoans := api.Group("/client/:client_id/loans")
		clientLoans.Use(auth.Middleware(verifier, permissions))
		{
			clientLoans.GET("", loanHandler.GetLoans)
			clientLoans.GET("/:loan_id", loanHandler.GetLoanByID)
			clientLoans.POST("/request", loanHandler.SubmitLoanRequest)
		}
	}
}

func RegisterServerLifecycle(lc fx.Lifecycle, server *http.Server) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				if err := server.ListenAndServe(); err != nil && !stderrors.Is(err, http.ErrServerClosed) {
					log.Fatal(err)
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return server.Shutdown(ctx)
		},
	})
}
