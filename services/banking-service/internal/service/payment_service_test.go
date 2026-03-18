package service

import (
	"banking-service/internal/dto"
	"banking-service/internal/model"
	"banking-service/internal/repository"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// ── Fake Payment Repo ──────────────────────────────────────────────

type fakePaymentRepo struct {
	createErr error
	getErr    error
	payment   *model.Payment
}

func (f *fakePaymentRepo) Create(ctx context.Context, p *model.Payment) error {
	if f.createErr != nil {
		return f.createErr
	}
	p.PaymentID = 1
	f.payment = p
	return nil
}

func (f *fakePaymentRepo) GetByID(ctx context.Context, id uint) (*model.Payment, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.payment, nil
}

func (f *fakePaymentRepo) Update(ctx context.Context, p *model.Payment) error {
	f.payment = p
	return nil
}

type fakeTransactionRepo struct {
	createErr     error
	getErr        error
	updateErr     error
	transaction   *model.Transaction
	returnedTx    *model.Transaction
	returnedTxErr error
}

func (f *fakeTransactionRepo) Create(_ context.Context, t *model.Transaction) error {
	if f.createErr != nil {
		return f.createErr
	}
	t.TransactionID = 1 // simulate ID assignment
	f.transaction = t
	return nil
}

func (f *fakeTransactionRepo) Update(ctx context.Context, t *model.Transaction) error {
	f.transaction = t
	return nil
}

func (f *fakeTransactionRepo) GetByID(_ context.Context, _ uint) (*model.Transaction, error) {
	// return preset transaction or error
	return f.returnedTx, f.returnedTxErr
}

// ── Fake Payment Account Repo ──────────────────────────────────────

type fakePaymentAccountRepo struct {
	accounts map[string]*model.Account
	findErr  error
}

func newFakePaymentAccountRepo(accounts ...*model.Account) *fakePaymentAccountRepo {
	m := make(map[string]*model.Account)
	for _, a := range accounts {
		m[a.AccountNumber] = a
	}
	return &fakePaymentAccountRepo{accounts: m}
}

func (f *fakePaymentAccountRepo) Create(ctx context.Context, account *model.Account) error {
	return nil
}

func (f *fakePaymentAccountRepo) AccountNumberExists(ctx context.Context, accountNumber string) (bool, error) {
	_, exists := f.accounts[accountNumber]
	return exists, nil
}

func (f *fakePaymentAccountRepo) FindByAccountNumber(ctx context.Context, accountNumber string) (*model.Account, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	acc, exists := f.accounts[accountNumber]
	if !exists {
		return nil, errors.New("account not found")
	}
	return acc, nil
}

func (f *fakePaymentAccountRepo) UpdateBalance(ctx context.Context, account *model.Account) error {
	f.accounts[account.AccountNumber] = account
	return nil
}

func (f *fakePaymentAccountRepo) NameExistsForClient(_ context.Context, _ uint, _ string, _ string) (bool, error) {
	return false, nil
}

// ── Fake Exchange Service ──────────────────────────────────────────

type fakeExchangeService struct {
	rate float64
	err  error
}

func (f *fakeExchangeService) Convert(ctx context.Context, amount float64, from, to model.CurrencyCode) (float64, error) {
	if f.err != nil {
		return 0, f.err
	}
	return amount * f.rate, nil
}

// ── Constructor ────────────────────────────────────────────────────────

func newTestPaymentService(
	paymentRepo repository.PaymentRepository,
	transactionRepo repository.TransactionRepository,
	accountRepo repository.AccountRepository,
	exchangeSvc CurrencyConverter,
) *PaymentService {
	return &PaymentService{
		paymentRepo:     paymentRepo,
		transactionRepo: transactionRepo,
		accountRepo:     accountRepo,
		exchangeService: exchangeSvc,
	}
}

// ── Tests ──────────────────────────────────────────────────────────

func TestCreatePayment_Success(t *testing.T) {
	payerAccount := &model.Account{
		AccountNumber:    "87654321",
		ClientID:         1,
		Balance:          10000,
		AvailableBalance: 10000,
		DailyLimit:       250000,
		MonthlyLimit:     1000000,
		DailySpending:    0,
		MonthlySpending:  0,
		Currency:         model.Currency{Code: model.RSD},
	}
	recipientAccount := &model.Account{
		AccountNumber: "12345678",
		ClientID:      2,
		Currency:      model.Currency{Code: model.RSD},
	}

	svc := newTestPaymentService(
		&fakePaymentRepo{},
		&fakeTransactionRepo{},
		newFakePaymentAccountRepo(payerAccount, recipientAccount),
		&fakeExchangeService{rate: 1.0},
	)

	req := dto.CreatePaymentRequest{
		RecipientName:          "John Doe",
		RecipientAccountNumber: "12345678",
		Amount:                 100,
		PayerAccountNumber:     "87654321",
	}

	payment, err := svc.CreatePayment(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, "John Doe", payment.RecipientName)
}

func TestCreatePayment_InsufficientFunds(t *testing.T) {
	payerAccount := &model.Account{
		AccountNumber:    "87654321",
		ClientID:         1,
		Balance:          50,
		AvailableBalance: 50,
		DailyLimit:       250000,
		MonthlyLimit:     1000000,
		Currency:         model.Currency{Code: model.RSD},
	}
	recipientAccount := &model.Account{
		AccountNumber: "12345678",
		ClientID:      2,
		Currency:      model.Currency{Code: model.RSD},
	}

	svc := newTestPaymentService(
		&fakePaymentRepo{},
		&fakeTransactionRepo{},
		newFakePaymentAccountRepo(payerAccount, recipientAccount),
		&fakeExchangeService{rate: 1.0},
	)

	req := dto.CreatePaymentRequest{
		RecipientName:          "John Doe",
		RecipientAccountNumber: "12345678",
		Amount:                 100,
		PayerAccountNumber:     "87654321",
	}

	payment, err := svc.CreatePayment(context.Background(), req)
	require.Nil(t, payment)
	require.Error(t, err)
	require.Contains(t, err.Error(), "insufficient funds")
}

func TestCreatePayment_DailyLimitExceeded(t *testing.T) {
	payerAccount := &model.Account{
		AccountNumber:    "87654321",
		ClientID:         1,
		Balance:          500000,
		AvailableBalance: 500000,
		DailyLimit:       1000,
		MonthlyLimit:     1000000,
		DailySpending:    900,
		MonthlySpending:  0,
		Currency:         model.Currency{Code: model.RSD},
	}
	recipientAccount := &model.Account{
		AccountNumber: "12345678",
		ClientID:      2,
		Currency:      model.Currency{Code: model.RSD},
	}

	svc := newTestPaymentService(
		&fakePaymentRepo{},
		&fakeTransactionRepo{},
		newFakePaymentAccountRepo(payerAccount, recipientAccount),
		&fakeExchangeService{rate: 1.0},
	)

	req := dto.CreatePaymentRequest{
		RecipientName:          "John Doe",
		RecipientAccountNumber: "12345678",
		Amount:                 200,
		PayerAccountNumber:     "87654321",
	}

	payment, err := svc.CreatePayment(context.Background(), req)
	require.Nil(t, payment)
	require.Error(t, err)
	require.Contains(t, err.Error(), "daily limit exceeded")
}

func TestCreatePayment_MonthlyLimitExceeded(t *testing.T) {
	payerAccount := &model.Account{
		AccountNumber:    "87654321",
		ClientID:         1,
		Balance:          500000,
		AvailableBalance: 500000,
		DailyLimit:       1000000,
		MonthlyLimit:     1000,
		DailySpending:    0,
		MonthlySpending:  900,
		Currency:         model.Currency{Code: model.RSD},
	}
	recipientAccount := &model.Account{
		AccountNumber: "12345678",
		ClientID:      2,
		Currency:      model.Currency{Code: model.RSD},
	}

	svc := newTestPaymentService(
		&fakePaymentRepo{},
		&fakeTransactionRepo{},
		newFakePaymentAccountRepo(payerAccount, recipientAccount),
		&fakeExchangeService{rate: 1.0},
	)

	req := dto.CreatePaymentRequest{
		RecipientName:          "John Doe",
		RecipientAccountNumber: "12345678",
		Amount:                 200,
		PayerAccountNumber:     "87654321",
	}

	payment, err := svc.CreatePayment(context.Background(), req)
	require.Nil(t, payment)
	require.Error(t, err)
	require.Contains(t, err.Error(), "monthly limit exceeded")
}

func TestCreatePayment_RecipientNotFound(t *testing.T) {
	payerAccount := &model.Account{
		AccountNumber:    "87654321",
		ClientID:         1,
		Balance:          10000,
		AvailableBalance: 10000,
		DailyLimit:       250000,
		MonthlyLimit:     1000000,
		Currency:         model.Currency{Code: model.RSD},
	}

	svc := newTestPaymentService(
		&fakePaymentRepo{},
		&fakeTransactionRepo{},
		newFakePaymentAccountRepo(payerAccount),
		&fakeExchangeService{rate: 1.0},
	)

	req := dto.CreatePaymentRequest{
		RecipientName:          "John Doe",
		RecipientAccountNumber: "99999999",
		Amount:                 100,
		PayerAccountNumber:     "87654321",
	}

	payment, err := svc.CreatePayment(context.Background(), req)
	require.Nil(t, payment)
	require.Error(t, err)
}

func TestCreatePayment_TransactionRepoError(t *testing.T) {
	payerAccount := &model.Account{
		AccountNumber:    "87654321",
		ClientID:         1,
		Balance:          10000,
		AvailableBalance: 10000,
		DailyLimit:       250000,
		MonthlyLimit:     1000000,
		Currency:         model.Currency{Code: model.RSD},
	}
	recipientAccount := &model.Account{
		AccountNumber: "12345678",
		ClientID:      2,
		Currency:      model.Currency{Code: model.RSD},
	}

	svc := newTestPaymentService(
		&fakePaymentRepo{},
		&fakeTransactionRepo{createErr: errors.New("db error")},
		newFakePaymentAccountRepo(payerAccount, recipientAccount),
		&fakeExchangeService{rate: 1.0},
	)

	req := dto.CreatePaymentRequest{
		RecipientName:          "John Doe",
		RecipientAccountNumber: "12345678",
		Amount:                 100,
		PayerAccountNumber:     "87654321",
	}

	payment, err := svc.CreatePayment(context.Background(), req)
	require.Nil(t, payment)
	require.Error(t, err)
}
