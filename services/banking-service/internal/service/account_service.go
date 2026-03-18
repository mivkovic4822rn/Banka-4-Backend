package service

import (
	"banking-service/internal/client"
	"banking-service/internal/dto"
	"banking-service/internal/model"
	"banking-service/internal/repository"
	"common/pkg/errors"
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	mathrand "math/rand"
	"time"

	"gorm.io/gorm"
)

type AccountService struct {
	repo             repository.AccountRepository
	verificationRepo repository.VerificationTokenRepository
	userClient       client.UserClient
	db               *gorm.DB
}

func NewAccountService(
	repo repository.AccountRepository,
	verificationRepo repository.VerificationTokenRepository,
	userClient client.UserClient,
	db *gorm.DB,
) *AccountService {
	return &AccountService{
		repo:             repo,
		verificationRepo: verificationRepo,
		userClient:       userClient,
		db:               db,
	}
}

func (s *AccountService) generateAccountNumber(typeCode string) string {
	random := fmt.Sprintf("%09d", mathrand.Intn(1_000_000_000))
	return model.BankCode + model.BranchCode + random + typeCode
}

func (s *AccountService) isValidAccountNumber(ctx context.Context, number string) bool {
	var exists, _ = s.repo.AccountNumberExists(ctx, number)

	if exists {
		return false
	}

	// TODO Actually implement checksum
	sum := 0
	for _, ch := range number {
		sum += int(ch - '0')
	}
	return sum%11 != 0
}

func (s *AccountService) generateValidAccountNumber(ctx context.Context, accountKind model.AccountKind, accountType model.AccountType, subtype model.Subtype) string {
	typeCode := model.GetTypeCode(accountKind, accountType, subtype)
	for {
		number := s.generateAccountNumber(typeCode)
		if s.isValidAccountNumber(ctx, number) {
			return number
		}
	}
}

func (s *AccountService) findCurrencyByCode(ctx context.Context, code model.CurrencyCode) (*model.Currency, error) {
	var currency model.Currency
	result := s.db.WithContext(ctx).Where("code = ?", code).First(&currency)
	if result.Error != nil {
		return nil, errors.NotFoundErr("currency not found: " + string(code))
	}
	return &currency, nil
}

func (s *AccountService) Create(ctx context.Context, req dto.CreateAccountRequest) (*model.Account, error) {
	if _, err := s.userClient.GetClientByID(ctx, req.ClientID); err != nil {
		return nil, errors.NotFoundErr("client not found")
	}

	if _, err := s.userClient.GetEmployeeByID(ctx, req.EmployeeID); err != nil {
		return nil, errors.NotFoundErr("employee not found")
	}

	if req.AccountType == model.AccountTypeBusiness && req.CompanyID == nil {
		return nil, errors.BadRequestErr("business account requires a company")
	}

	if req.AccountType == model.AccountTypePersonal && req.CompanyID != nil {
		return nil, errors.BadRequestErr("personal account cannot have a company")
	}

	currencyCode := model.CurrencyCode("RSD")
	if req.AccountKind == model.AccountKindForeign {
		if req.CurrencyCode == "" {
			return nil, errors.BadRequestErr("currency code is required for foreign accounts")
		}
		currencyCode = req.CurrencyCode
	}

	if req.AccountKind == model.AccountKindCurrent && req.Subtype == "" {
		return nil, errors.BadRequestErr("subtype is required for current accounts")
	}

	currency, err := s.findCurrencyByCode(ctx, currencyCode)
	if err != nil {
		return nil, err
	}

	dailyLimit := model.DefaultDailyLimitRSD
	monthlyLimit := model.DefaultMonthlyLimitRSD
	if req.AccountKind == model.AccountKindForeign {
		// TODO Use Exchange Office to change limits.
	}

	account := &model.Account{
		AccountNumber:    s.generateValidAccountNumber(ctx, req.AccountKind, req.AccountType, req.Subtype),
		Name:             req.Name,
		ClientID:         req.ClientID,
		EmployeeID:       req.EmployeeID,
		CompanyID:        req.CompanyID,
		Balance:          req.InitialBalance,
		AvailableBalance: req.InitialBalance,
		ExpiresAt:        req.ExpiresAt,
		CurrencyID:       currency.CurrencyID,
		AccountType:      req.AccountType,
		AccountKind:      req.AccountKind,
		Subtype:          req.Subtype,
		DailyLimit:       dailyLimit,
		MonthlyLimit:     monthlyLimit,
	}

	if err := s.repo.Create(ctx, account); err != nil {
		return nil, errors.InternalErr(err)
	}

	return account, nil
}

func (s *AccountService) GetClientAccounts(ctx context.Context, clientID uint) ([]model.Account, error) {
	accounts, err := s.repo.FindAllByClientID(ctx, clientID)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	return accounts, nil
}

func (s *AccountService) GetAccountDetails(ctx context.Context, accountNumber string, clientID uint) (*model.Account, error) {
	account, err := s.repo.FindByAccountNumberAndClientID(ctx, accountNumber, clientID)
	if err != nil {
		return nil, errors.NotFoundErr("account not found")
	}
	return account, nil
}

func (s *AccountService) UpdateAccountName(ctx context.Context, accountNumber string, clientID uint, name string) error {
	account, err := s.repo.FindByAccountNumberAndClientID(ctx, accountNumber, clientID)
	if err != nil {
		return errors.NotFoundErr("account not found")
	}

	if account.Name == name {
		return errors.BadRequestErr("new name is the same as the current name")
	}

	exists, err := s.repo.NameExistsForClient(ctx, clientID, name, accountNumber)
	if err != nil {
		return errors.InternalErr(err)
	}
	if exists {
		return errors.ConflictErr("an account with this name already exists")
	}

	if err := s.repo.UpdateName(ctx, accountNumber, name); err != nil {
		return errors.InternalErr(err)
	}
	return nil
}

func (s *AccountService) RequestLimitsChange(ctx context.Context, accountNumber string, clientID uint, daily float64, monthly float64) (string, error) {
	if _, err := s.repo.FindByAccountNumberAndClientID(ctx, accountNumber, clientID); err != nil {
		return "", errors.NotFoundErr("account not found")
	}

	if err := s.verificationRepo.DeleteByAccountAndClient(ctx, accountNumber, clientID); err != nil {
		return "", errors.InternalErr(err)
	}

	code, err := generateSixDigitCode()
	if err != nil {
		return "", errors.InternalErr(err)
	}

	token := &model.VerificationToken{
		ClientID:        clientID,
		AccountNumber:   accountNumber,
		Code:            code,
		NewDailyLimit:   daily,
		NewMonthlyLimit: monthly,
		ExpiresAt:       time.Now().Add(5 * time.Minute),
	}
	if err := s.verificationRepo.Create(ctx, token); err != nil {
		return "", errors.InternalErr(err)
	}

	return code, nil
}

func (s *AccountService) ConfirmLimitsChange(ctx context.Context, accountNumber string, clientID uint, code string) error {

	token, err := s.verificationRepo.FindByAccountAndClient(ctx, accountNumber, clientID)
	if err != nil {
		return errors.NotFoundErr("no pending limits change for this account")
	}

	if time.Now().After(token.ExpiresAt) {
		return errors.BadRequestErr("verification code has expired")
	}

	if code == "1234" { //cheat code for debug until mobile verification is implemented
	} else if token.Code != code {
		return errors.BadRequestErr("invalid verification code")
	}

	if err := s.repo.UpdateLimits(ctx, accountNumber, token.NewDailyLimit, token.NewMonthlyLimit); err != nil {
		return errors.InternalErr(err)
	}
	if err := s.verificationRepo.DeleteByAccountAndClient(ctx, accountNumber, clientID); err != nil {
		return errors.InternalErr(err)
	}
	return nil
}

func generateSixDigitCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n), nil
}
