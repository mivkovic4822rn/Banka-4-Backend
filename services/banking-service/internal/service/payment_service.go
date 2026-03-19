package service

import (
	"banking-service/internal/client"
	"banking-service/internal/dto"
	"banking-service/internal/model"
	"banking-service/internal/repository"
	"common/pkg/errors"
	"context"
	"time"
)

type paymentTransactionProcessor interface {
	Process(ctx context.Context, transactionID uint) error
}

type PaymentService struct {
	paymentRepo          repository.PaymentRepository
	transactionRepo      repository.TransactionRepository
	accountRepo          repository.AccountRepository
	mobileSecretClient   client.MobileSecretClient
	exchangeService      CurrencyConverter
	transactionProcessor paymentTransactionProcessor
	now                  func() time.Time
}

func NewPaymentService(
	paymentRepo repository.PaymentRepository,
	transactionRepo repository.TransactionRepository,
	accountRepo repository.AccountRepository,
	mobileSecretClient client.MobileSecretClient,
	exchangeService CurrencyConverter,
	transactionProcessor *TransactionProcessor,
) *PaymentService {
	return &PaymentService{
		paymentRepo:          paymentRepo,
		transactionRepo:      transactionRepo,
		accountRepo:          accountRepo,
		mobileSecretClient:   mobileSecretClient,
		exchangeService:      exchangeService,
		transactionProcessor: transactionProcessor,
		now:                  time.Now,
	}
}

func (s *PaymentService) CreatePayment(ctx context.Context, req dto.CreatePaymentRequest) (*model.Payment, error) {

	// Proveri da payer racun postoji
	payerAccount, err := s.accountRepo.FindByAccountNumber(ctx, req.PayerAccountNumber)
	if err != nil {
		return nil, errors.NotFoundErr("payer account not found")
	}

	// Proveri da recipient racun postoji
	recipientAccount, err := s.accountRepo.FindByAccountNumber(ctx, req.RecipientAccountNumber)
	if err != nil {
		return nil, errors.NotFoundErr("recipient account not found")
	}

	if recipientAccount.ClientID == payerAccount.ClientID {
		return nil, errors.BadRequestErr("cannot make payment for same client accounts, that is a transfer")
	}

	// Proveri dovoljno sredstava
	if payerAccount.AvailableBalance < req.Amount {
		return nil, errors.BadRequestErr("insufficient funds")
	}

	// Proveri dnevni limit
	if payerAccount.DailySpending+req.Amount > payerAccount.DailyLimit {
		return nil, errors.BadRequestErr("daily limit exceeded")
	}

	// Proveri mesecni limit
	if payerAccount.MonthlySpending+req.Amount > payerAccount.MonthlyLimit {
		return nil, errors.BadRequestErr("monthly limit exceeded")
	}

	// Konverzija valuta ako su razlicite
	endAmount := req.Amount
	endCurrencyCode := payerAccount.Currency.Code

	if payerAccount.Currency.Code != recipientAccount.Currency.Code {
		converted, err := s.exchangeService.Convert(ctx, req.Amount, payerAccount.Currency.Code, recipientAccount.Currency.Code)
		if err != nil {
			return nil, errors.InternalErr(err)
		}
		endAmount = converted
		endCurrencyCode = recipientAccount.Currency.Code
	}

	transaction := &model.Transaction{
		PayerAccountNumber:     req.PayerAccountNumber,
		RecipientAccountNumber: req.RecipientAccountNumber,
		StartAmount:            req.Amount,
		StartCurrencyCode:      payerAccount.Currency.Code,
		EndAmount:              endAmount,
		EndCurrencyCode:        endCurrencyCode,
		Status:                 model.TransactionProcessing,
	}

	if err := s.transactionRepo.Create(ctx, transaction); err != nil {
		return nil, errors.InternalErr(err)
	}

	payment := &model.Payment{
		TransactionID:   transaction.TransactionID,
		RecipientName:   req.RecipientName,
		ReferenceNumber: req.ReferenceNumber,
		PaymentCode:     req.PaymentCode,
		Purpose:         req.Purpose,
	}

	if err := s.paymentRepo.Create(ctx, payment); err != nil {
		return nil, errors.InternalErr(err)
	}

	return payment, nil
}

func (s *PaymentService) VerifyPayment(ctx context.Context, id uint, code, authorizationHeader string) (*model.Payment, error) {
	payment, err := s.paymentRepo.GetByID(ctx, id)
	if err != nil {
		return nil, errors.NotFoundErr("payment not found")
	}

	transaction := &payment.Transaction
	if transaction.Status != model.TransactionProcessing {
		return nil, errors.BadRequestErr("payment already processed")
	}

	secret, err := s.mobileSecretClient.GetMobileSecret(ctx, authorizationHeader)
	if err != nil {
		return nil, errors.ServiceUnavailableErr(err)
	}

	if !verifyTOTPCode(secret, code, s.now(), totpAllowedSkew) {
		return nil, errors.BadRequestErr("invalid verification code")
	}

	// Process transaction
	err = s.transactionProcessor.Process(ctx, transaction.TransactionID)
	if err != nil {
		return nil, err
	}

	return payment, nil
}
