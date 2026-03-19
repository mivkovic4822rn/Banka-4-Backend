package repository

import (
	"banking-service/internal/dto"
	"banking-service/internal/model"
	"context"
	"time"
)

type PaymentFilter struct {
	DateFrom  *time.Time
	DateTo    *time.Time
	AmountMin *float64
	AmountMax *float64
	Status    *model.TransactionStatus
}

type PaymentRepository interface {
	Create(ctx context.Context, payment *model.Payment) error
	GetByID(ctx context.Context, id uint) (*model.Payment, error)
	Update(ctx context.Context, payment *model.Payment) error
	FindAllByClientID(ctx context.Context, clientID uint, filter PaymentFilter) ([]model.Payment, error)
	FindByAccount(ctx context.Context, accountNumber string, filters *dto.PaymentFilters) ([]model.Payment, int64, error)
}
