package service

import (
	"banking-service/internal/dto"
	"banking-service/internal/model"
	"common/pkg/auth"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// ── Fake Payee Repo ────────────────────────────────────────────────

type fakePayeeRepo struct {
	payees    map[uint]*model.Payee
	createErr error
	updateErr error
	deleteErr error
	nextID    uint
}

func newFakePayeeRepo(payees ...*model.Payee) *fakePayeeRepo {
	m := make(map[uint]*model.Payee)
	var maxID uint
	for _, p := range payees {
		m[p.PayeeID] = p
		if p.PayeeID > maxID {
			maxID = p.PayeeID
		}
	}
	return &fakePayeeRepo{payees: m, nextID: maxID + 1}
}

func (f *fakePayeeRepo) FindAllByClientID(ctx context.Context, clientID uint) ([]model.Payee, error) {
	var result []model.Payee
	for _, p := range f.payees {
		if p.ClientID == clientID {
			result = append(result, *p)
		}
	}
	return result, nil
}

func (f *fakePayeeRepo) FindByID(ctx context.Context, id uint) (*model.Payee, error) {
	p, ok := f.payees[id]
	if !ok {
		return nil, nil
	}
	return p, nil
}

func (f *fakePayeeRepo) Create(ctx context.Context, payee *model.Payee) error {
	if f.createErr != nil {
		return f.createErr
	}
	payee.PayeeID = f.nextID
	f.nextID++
	f.payees[payee.PayeeID] = payee
	return nil
}

func (f *fakePayeeRepo) Update(ctx context.Context, payee *model.Payee) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	f.payees[payee.PayeeID] = payee
	return nil
}

func (f *fakePayeeRepo) Delete(ctx context.Context, id uint) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	delete(f.payees, id)
	return nil
}

// ── Helper ─────────────────────────────────────────────────────────

func ctxWithClient(clientID uint) context.Context {
	id := clientID
	return auth.SetAuthOnContext(context.Background(), &auth.AuthContext{
		ClientID: &id,
	})
}

// ── GetAll Tests ───────────────────────────────────────────────────

func TestGetAll_Success(t *testing.T) {
	repo := newFakePayeeRepo(
		&model.Payee{PayeeID: 1, ClientID: 1, Name: "Ana", AccountNumber: "111"},
		&model.Payee{PayeeID: 2, ClientID: 1, Name: "Marko", AccountNumber: "222"},
		&model.Payee{PayeeID: 3, ClientID: 2, Name: "Drugi klijent", AccountNumber: "333"},
	)
	svc := NewPayeeService(repo)

	payees, err := svc.GetAll(ctxWithClient(1))
	require.NoError(t, err)
	require.Len(t, payees, 2)
}

func TestGetAll_Unauthorized(t *testing.T) {
	svc := NewPayeeService(newFakePayeeRepo())

	payees, err := svc.GetAll(context.Background())
	require.Nil(t, payees)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not authenticated as client")
}

// ── Create Tests ───────────────────────────────────────────────────

func TestCreate_Success(t *testing.T) {
	svc := NewPayeeService(newFakePayeeRepo())

	payee, err := svc.Create(ctxWithClient(1), dto.CreatePayeeRequest{
		Name:          "Stefan",
		AccountNumber: "444000112345678913",
	})
	require.NoError(t, err)
	require.Equal(t, "Stefan", payee.Name)
	require.Equal(t, uint(1), payee.ClientID)
}

func TestCreate_Unauthorized(t *testing.T) {
	svc := NewPayeeService(newFakePayeeRepo())

	payee, err := svc.Create(context.Background(), dto.CreatePayeeRequest{
		Name:          "Stefan",
		AccountNumber: "444000112345678913",
	})
	require.Nil(t, payee)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not authenticated as client")
}

func TestCreate_RepoError(t *testing.T) {
	repo := newFakePayeeRepo()
	repo.createErr = errors.New("db error")
	svc := NewPayeeService(repo)

	payee, err := svc.Create(ctxWithClient(1), dto.CreatePayeeRequest{
		Name:          "Stefan",
		AccountNumber: "444000112345678913",
	})
	require.Nil(t, payee)
	require.Error(t, err)
}

// ── Update Tests ───────────────────────────────────────────────────

func TestUpdate_Success(t *testing.T) {
	repo := newFakePayeeRepo(
		&model.Payee{PayeeID: 1, ClientID: 1, Name: "Staro ime", AccountNumber: "111"},
	)
	svc := NewPayeeService(repo)

	payee, err := svc.Update(ctxWithClient(1), 1, dto.UpdatePayeeRequest{Name: "Novo ime"})
	require.NoError(t, err)
	require.Equal(t, "Novo ime", payee.Name)
	require.Equal(t, "111", payee.AccountNumber)
}

func TestUpdate_NotFound(t *testing.T) {
	svc := NewPayeeService(newFakePayeeRepo())

	payee, err := svc.Update(ctxWithClient(1), 99, dto.UpdatePayeeRequest{Name: "Novo ime"})
	require.Nil(t, payee)
	require.Error(t, err)
	require.Contains(t, err.Error(), "payee not found")
}

func TestUpdate_Forbidden(t *testing.T) {
	repo := newFakePayeeRepo(
		&model.Payee{PayeeID: 1, ClientID: 2, Name: "Tudji", AccountNumber: "111"},
	)
	svc := NewPayeeService(repo)

	payee, err := svc.Update(ctxWithClient(1), 1, dto.UpdatePayeeRequest{Name: "Pokusaj"})
	require.Nil(t, payee)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not your payee")
}

func TestUpdate_Unauthorized(t *testing.T) {
	svc := NewPayeeService(newFakePayeeRepo())

	payee, err := svc.Update(context.Background(), 1, dto.UpdatePayeeRequest{Name: "Novo"})
	require.Nil(t, payee)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not authenticated as client")
}

// ── Delete Tests ───────────────────────────────────────────────────

func TestDelete_Success(t *testing.T) {
	repo := newFakePayeeRepo(
		&model.Payee{PayeeID: 1, ClientID: 1, Name: "Ana", AccountNumber: "111"},
	)
	svc := NewPayeeService(repo)

	err := svc.Delete(ctxWithClient(1), 1)
	require.NoError(t, err)
}

func TestDelete_NotFound(t *testing.T) {
	svc := NewPayeeService(newFakePayeeRepo())

	err := svc.Delete(ctxWithClient(1), 99)
	require.Error(t, err)
	require.Contains(t, err.Error(), "payee not found")
}

func TestDelete_Forbidden(t *testing.T) {
	repo := newFakePayeeRepo(
		&model.Payee{PayeeID: 1, ClientID: 2, Name: "Tudji", AccountNumber: "111"},
	)
	svc := NewPayeeService(repo)

	err := svc.Delete(ctxWithClient(1), 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not your payee")
}

func TestDelete_Unauthorized(t *testing.T) {
	svc := NewPayeeService(newFakePayeeRepo())

	err := svc.Delete(context.Background(), 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not authenticated as client")
}