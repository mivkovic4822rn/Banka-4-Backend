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
)

type AccountService struct {
	repo               repository.AccountRepository
	currencyRepo       repository.CurrencyRepository
	verificationRepo   repository.VerificationTokenRepository
	userClient         client.UserClient
	cardService        *CardService
	mobileSecretClient client.MobileSecretClient
	exchangeService    CurrencyConverter
}

func NewAccountService(
	repo repository.AccountRepository,
	currencyRepo repository.CurrencyRepository,
	verificationRepo repository.VerificationTokenRepository,
	userClient client.UserClient,
	cardService *CardService,
	mobileSecretClient client.MobileSecretClient,
	exchangeService CurrencyConverter,
) *AccountService {
	return &AccountService{
		repo:               repo,
		currencyRepo:       currencyRepo,
		verificationRepo:   verificationRepo,
		userClient:         userClient,
		cardService:        cardService,
		mobileSecretClient: mobileSecretClient,
		exchangeService:    exchangeService,
	}
}

func (s *AccountService) generateAccountNumber(typeCode string) string {
	random := fmt.Sprintf("%09d", mathrand.Intn(1_000_000_000))
	return model.BankCode + model.BranchCode + random + typeCode
}

func (s *AccountService) isValidAccountNumber(ctx context.Context, number string) bool {
	exists, _ := s.repo.AccountNumberExists(ctx, number)
	if exists {
		return false
	}

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

	currencyCode := model.RSD
	if req.AccountKind == model.AccountKindForeign {
		if req.CurrencyCode == "" {
			return nil, errors.BadRequestErr("currency code is required for foreign accounts")
		}
		currencyCode = req.CurrencyCode
	}

	if req.AccountKind == model.AccountKindCurrent && req.Subtype == "" {
		return nil, errors.BadRequestErr("subtype is required for current accounts")
	}

	exists, err := s.repo.NameExistsForClient(ctx, req.ClientID, req.Name, "")
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	if exists {
		return nil, errors.ConflictErr("account with this name already exists")
	}

	currency, err := s.currencyRepo.FindByCode(ctx, currencyCode)
	if err != nil {
		return nil, err
	}

	dailyLimit := model.DefaultDailyLimitRSD
	monthlyLimit := model.DefaultMonthlyLimitRSD
	if req.AccountKind == model.AccountKindForeign {
		convertedDaily, err := s.exchangeService.Convert(ctx, model.DefaultDailyLimitRSD, model.RSD, currencyCode)
		if err != nil {
			return nil, err
		}
		convertedMonthly, err := s.exchangeService.Convert(ctx, model.DefaultMonthlyLimitRSD, model.RSD, currencyCode)
		if err != nil {
			return nil, err
		}
		dailyLimit = convertedDaily
		monthlyLimit = convertedMonthly
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

	if req.GenerateCard {
		if _, err := s.cardService.createCard(ctx, account, nil); err != nil {
			return nil, err
		}
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

func (s *AccountService) ConfirmLimitsChange(ctx context.Context, accountNumber string, clientID uint, code, authorizationHeader string) error {

	token, err := s.verificationRepo.FindByAccountAndClient(ctx, accountNumber, clientID)
	if err != nil {
		return errors.NotFoundErr("no pending limits change for this account")
	}

	if time.Now().After(token.ExpiresAt) {
		return errors.BadRequestErr("verification code has expired")
	}

	secret, err := s.mobileSecretClient.GetMobileSecret(ctx, authorizationHeader)
	if err != nil {
		return errors.ServiceUnavailableErr(err)
	}

	if !verifyTOTPCode(secret, code, time.Now(), totpAllowedSkew) {
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
