package service

import (
	"banking-service/internal/dto"
	"banking-service/internal/model"
	"banking-service/internal/repository"
	"bytes"
	"common/pkg/errors"
	"context"
	"fmt"

	"github.com/go-pdf/fpdf"
)

type PaymentService struct {
	paymentRepo          repository.PaymentRepository
	transactionRepo      repository.TransactionRepository
	accountRepo          repository.AccountRepository
	exchangeService      CurrencyConverter
	transactionProcessor *TransactionProcessor
}

func NewPaymentService(
	paymentRepo          repository.PaymentRepository,
	transactionRepo      repository.TransactionRepository,
	accountRepo          repository.AccountRepository,
	exchangeService      CurrencyConverter,
	transactionProcessor *TransactionProcessor,
) *PaymentService {
	return &PaymentService{
		paymentRepo:          paymentRepo,
		transactionRepo:      transactionRepo,
		accountRepo:          accountRepo,
		exchangeService:      exchangeService,
		transactionProcessor: transactionProcessor,
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

	payment.Transaction = *transaction
	return payment, nil
}

func (s *PaymentService) GetPaymentByID(ctx context.Context, id uint) (*model.Payment, error) {
	payment, err := s.paymentRepo.GetByID(ctx, id)
	if err != nil {
		return nil, errors.NotFoundErr("payment not found")
	}

	payerAccount, err := s.accountRepo.FindByAccountNumber(ctx, payment.Transaction.PayerAccountNumber)
	if payerAccount == nil {
		return nil, errors.NotFoundErr("payer account not found")
	}
	if err != nil {
		return nil, errors.InternalErr(err)
	}

	return payment, nil
}

func (s *PaymentService) GenerateReceipt(ctx context.Context, id uint) ([]byte, error) {
	payment, err := s.GetPaymentByID(ctx, id)
	if err != nil {
		return nil, err
	}

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddPage()

	pdf.SetFont("Arial", "B", 20)
	pdf.Cell(0, 12, "Potvrda o placanju")
	pdf.Ln(16)

	pdf.SetFont("Arial", "", 12)
	pdf.Cell(60, 8, "Broj placanja:")
	pdf.Cell(0, 8, fmt.Sprintf("%d", payment.PaymentID))
	pdf.Ln(8)

	pdf.Cell(60, 8, "Datum:")
	pdf.Cell(0, 8, payment.Transaction.CreatedAt.Format("02.01.2006. 15:04"))
	pdf.Ln(8)

	pdf.Cell(60, 8, "Status:")
	pdf.Cell(0, 8, string(payment.Transaction.Status))
	pdf.Ln(8)

	pdf.Ln(4)
	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(0, 8, "Detalji placanja")
	pdf.Ln(10)

	pdf.SetFont("Arial", "", 12)
	pdf.Cell(60, 8, "Primalac:")
	pdf.Cell(0, 8, payment.RecipientName)
	pdf.Ln(8)

	pdf.Cell(60, 8, "Racun platioca:")
	pdf.Cell(0, 8, payment.Transaction.PayerAccountNumber)
	pdf.Ln(8)

	pdf.Cell(60, 8, "Racun primaoca:")
	pdf.Cell(0, 8, payment.Transaction.RecipientAccountNumber)
	pdf.Ln(8)

	pdf.Cell(60, 8, "Iznos:")
	pdf.Cell(0, 8, fmt.Sprintf("%.2f %s", payment.Transaction.StartAmount, payment.Transaction.StartCurrencyCode))
	pdf.Ln(8)

	pdf.Cell(60, 8, "Svrha placanja:")
	pdf.Cell(0, 8, payment.Purpose)
	pdf.Ln(8)

	pdf.Cell(60, 8, "Poziv na broj:")
	pdf.Cell(0, 8, payment.ReferenceNumber)
	pdf.Ln(8)

	pdf.Cell(60, 8, "Sifra placanja:")
	pdf.Cell(0, 8, payment.PaymentCode)
	pdf.Ln(8)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, errors.InternalErr(err)
	}

	return buf.Bytes(), nil
}
func (s *PaymentService) GetAccountPayments(ctx context.Context, accountNumber string, filters *dto.PaymentFilters) ([]model.Payment, int64, error) {
	payments, total, err := s.paymentRepo.FindByAccount(ctx, accountNumber, filters)
	if err != nil {
		return nil, 0, errors.InternalErr(err)
	}
	return payments, total, nil
}

func (s *PaymentService) GetClientPayments(ctx context.Context, clientID uint, filters *dto.PaymentFilters) ([]model.Payment, int64, error) {
	payments, total, err := s.paymentRepo.FindByClient(ctx, clientID, filters)
	if err != nil {
		return nil, 0, errors.InternalErr(err)
	}
	return payments, total, nil
}

func (s *PaymentService) VerifyPayment(ctx context.Context, id uint, code string) (*model.Payment, error) {
	payment, err := s.paymentRepo.GetByID(ctx, id)
	if err != nil {
		return nil, errors.NotFoundErr("payment not found")
	}
	if payment == nil {
		return nil, errors.NotFoundErr("payment not found")
	}

	transaction := &payment.Transaction
	if transaction.Status != model.TransactionProcessing {
		return nil, errors.BadRequestErr("payment already processed")
	}

	// TODO: mobile verification (#56) - verifikacija putem mobilne aplikacije

	// Process transaction
	err = s.transactionProcessor.Process(ctx, transaction.TransactionID)
	if err != nil {
		return nil, err
	}

	return payment, nil
}


