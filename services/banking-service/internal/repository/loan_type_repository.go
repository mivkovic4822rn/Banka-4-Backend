package repository

import (
	"banking-service/internal/model"
	"context"
)

type LoanTypeRepository interface {
	FindByID(ctx context.Context, id uint) (*model.LoanType, error)
}
