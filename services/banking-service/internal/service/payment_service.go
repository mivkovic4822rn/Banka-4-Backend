package service

import (
	"banking-service/internal/dto"
	"banking-service/internal/model"
	"banking-service/internal/repository"
	"common/pkg/errors"
	"context"
)

type PaymentService struct {
	repo repository.PaymentRepository
}

func NewPaymentService(repo repository.PaymentRepository) *PaymentService {
	return &PaymentService{repo: repo}
}

func (s *PaymentService) CreatePayment(ctx context.Context, req dto.CreatePaymentRequest) (*model.Payment, error) {

	// TODO: proveriti sredstva (#45)
	// TODO: proveriti limit
	// TODO: proveriti postojanje računa (#45)

	// TODO: currency conversion (#44)

	payment := &model.Payment{
		RecipientName:    req.RecipientName,
		RecipientAccount: req.RecipientAccountNumber,
		Amount:           req.Amount,
		ReferenceNumber:  req.ReferenceNumber,
		PaymentCode:      req.PaymentCode,
		Purpose:          req.Purpose,
		PayerAccount:     req.PayerAccount,
		Currency:         req.Currency,
		Status:           model.PaymentProcessing,
	}

	err := s.repo.Create(ctx, payment)
	if err != nil {
		return nil, errors.InternalErr(err)
	}

	return payment, nil
}

func (s *PaymentService) VerifyPayment(ctx context.Context, id uint, code string) (*model.Payment, error) {

	payment, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, errors.InternalErr(err)
	}

	// TODO: mobile verification

	if code == "1234" {
		payment.Status = model.PaymentCompleted

		// TODO: save recipient
	} else {
		payment.Status = model.PaymentRejected
	}

	err = s.repo.Update(ctx, payment)
	if err != nil {
		return nil, errors.InternalErr(err)
	}

	return payment, nil
}
