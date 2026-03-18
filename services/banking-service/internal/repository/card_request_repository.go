package repository

import (
	"banking-service/internal/model"
	"context"
)

type CardRequestRepository interface {
	Create(ctx context.Context, request *model.CardRequest) error
	FindByAccountNumberAndCode(ctx context.Context, accountNumber string, code string) (*model.CardRequest, error)
	FindLatestPendingByAccountNumber(ctx context.Context, accountNumber string) (*model.CardRequest, error)
	Update(ctx context.Context, request *model.CardRequest) error
}
