package repository

import (
	"banking-service/internal/model"
	"context"
)

type LoanRepository interface {
	CreateRequest(ctx context.Context, request *model.LoanRequest) error
	FindByClientID(ctx context.Context, clientID uint, sortByAmountDesc bool) ([]model.LoanRequest, error)
	FindByIDAndClientID(ctx context.Context, id uint, clientID uint) (*model.LoanRequest, error)
}
