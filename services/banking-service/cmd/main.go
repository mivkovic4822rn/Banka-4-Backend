package main

import (
	"banking-service/internal/client"
	clientgrpc "banking-service/internal/client/grpc"
	"banking-service/internal/config"
	"banking-service/internal/handler"
	"banking-service/internal/model"
	"banking-service/internal/permission"
	"banking-service/internal/repository"
	"banking-service/internal/seed"
	"banking-service/internal/server"
	"banking-service/internal/service"
	"common/pkg/auth"
	"common/pkg/db"
	"common/pkg/jwt"
	"common/pkg/logging"
	"common/pkg/pb"

	"go.uber.org/fx"
	"google.golang.org/grpc"
	"gorm.io/gorm"
)

// @title Banking Service API
// @version 1.0
// @description API for managing accounts, balances, and banking operations.
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description JWT Authorization header using the Bearer scheme.
func main() {
	fx.New(
		fx.Provide(
			config.Load,
			func(cfg *config.Configuration) (*gorm.DB, error) {
				return db.New(cfg.DB.DSN())
			},
			func(cfg *config.Configuration) auth.TokenVerifier {
				return jwt.NewJWTVerifier(cfg.JWTSecret)
			},
			client.NewUserServiceConnection,
			fx.Annotate(
				clientgrpc.NewUserServiceClient,
				fx.As(new(client.UserClient)),
			),
			func(conn *grpc.ClientConn) pb.PermissionServiceClient {
				return pb.NewPermissionServiceClient(conn)
			},
			func(c pb.PermissionServiceClient) auth.PermissionProvider {
				return permission.NewGrpcPermissionProvider(c)
			},
			handler.NewHealthHandler,
			repository.NewAccountRepository,
			repository.NewCompanyRepository,
			repository.NewPaymentRepository,
			service.NewAccountService,
			service.NewCompanyService,
			service.NewPaymentService,
			handler.NewAccountHandler,
			handler.NewCompanyHandler,
			handler.NewPaymentHandler,
		),
		fx.Invoke(func(cfg *config.Configuration) error {
			return logging.Init(cfg.Env)
		}),
		fx.Invoke(func(db *gorm.DB) error {
			if err := db.AutoMigrate(
				&model.Currency{},
				&model.WorkCode{},
				&model.Company{},
				&model.Account{},
				&model.Payment{},
			); err != nil {
				return err
			}
			return seed.Run(db)
		}),
		fx.Invoke(server.NewServer),
	).Run()
}
