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

// ── Fake Repo ────────────────────────────────────────────────────────

type fakePaymentRepo struct {
	createErr error
	getErr    error
	payment   *model.Payment
}

func (f *fakePaymentRepo) Create(ctx context.Context, p *model.Payment) error {
	if f.createErr != nil {
		return f.createErr
	}
	p.ID = 1
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

// ── Constructor ────────────────────────────────────────────────────────

func newPaymentService(repo repository.PaymentRepository) *PaymentService {
	return &PaymentService{repo: repo}
}

// ── Tests ──────────────────────────────────────────────────────────────

func TestCreatePayment(t *testing.T) {
	repo := &fakePaymentRepo{}
	svc := newPaymentService(repo)

	req := dto.CreatePaymentRequest{
		RecipientName:          "John Doe",
		RecipientAccountNumber: "12345678",
		Amount:                 100,
		PayerAccount:           "87654321",
		Currency:               "RSD",
	}

	payment, err := svc.CreatePayment(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, model.PaymentProcessing, payment.Status)
	require.Equal(t, "John Doe", payment.RecipientName)
}

func TestVerifyPayment_Success(t *testing.T) {
	repo := &fakePaymentRepo{
		payment: &model.Payment{ID: 1, Status: model.PaymentProcessing},
	}
	svc := newPaymentService(repo)

	p, err := svc.VerifyPayment(context.Background(), 1, "1234")
	require.NoError(t, err)
	require.Equal(t, model.PaymentCompleted, p.Status)
}

func TestVerifyPayment_Rejected(t *testing.T) {
	repo := &fakePaymentRepo{
		payment: &model.Payment{ID: 1, Status: model.PaymentProcessing},
	}
	svc := newPaymentService(repo)

	p, err := svc.VerifyPayment(context.Background(), 1, "0000")
	require.NoError(t, err)
	require.Equal(t, model.PaymentRejected, p.Status)
}

func TestCreatePayment_Error(t *testing.T) {
	repo := &fakePaymentRepo{createErr: errors.New("db error")}
	svc := newPaymentService(repo)

	req := dto.CreatePaymentRequest{
		RecipientName:          "John Doe",
		RecipientAccountNumber: "12345678",
		Amount:                 100,
		PayerAccount:           "87654321",
		Currency:               "RSD",
	}

	p, err := svc.CreatePayment(context.Background(), req)
	require.Nil(t, p)
	require.Error(t, err)
	require.Equal(t, "db error", err.Error())
}
