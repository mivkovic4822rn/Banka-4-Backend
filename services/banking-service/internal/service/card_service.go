package service

import (
	"banking-service/internal/client"
	"banking-service/internal/model"
	"banking-service/internal/repository"
	"common/pkg/auth"
	"common/pkg/errors"
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"time"
)

const (
	visaIIN              = "453275"
	masterCardIIN        = "532451"
	dinaCardIIN          = "989100"
	cardNumberLength     = 16
	cardCVVLength        = 3
	confirmationCodeSize = 6
	cardExpiryYears      = 4
	confirmationCodeTTL  = 15 * time.Minute
)

type AuthorizedPersonInput struct {
	FirstName   string
	LastName    string
	DateOfBirth time.Time
	Gender      string
	Email       string
	PhoneNumber string
	Address     string
}

type RequestCardInput struct {
	AccountNumber     string
	AuthorizedPerson  *AuthorizedPersonInput
}

type AccountCardsResult struct {
	Account *model.Account
	Cards   []model.Card
}
type CardService struct {
	accountRepo          repository.AccountRepository
	cardRepo             repository.CardRepository
	authorizedPersonRepo repository.AuthorizedPersonRepository
	cardRequestRepo      repository.CardRequestRepository
	userClient           client.UserClient
	mailer 				 Mailer
}

func NewCardService(
	accountRepo repository.AccountRepository,
	cardRepo repository.CardRepository,
	authorizedPersonRepo repository.AuthorizedPersonRepository,
	cardRequestRepo repository.CardRequestRepository,
	userClient client.UserClient,
	mailer Mailer,
) *CardService {
	return &CardService{
		accountRepo:          accountRepo,
		cardRepo:             cardRepo,
		authorizedPersonRepo: authorizedPersonRepo,
		cardRequestRepo:      cardRequestRepo,
		userClient:           userClient,
		mailer:               mailer,
	}
}

func GenerateCardBrand() (model.CardBrand, error) {
	brands := []model.CardBrand{
		model.CardBrandVisa,
		model.CardBrandMasterCard,
		model.CardBrandDinaCard,
	}

	index, err := randomInt(len(brands))
	if err != nil {
		return "", err
	}

	return brands[index], nil
}

func GenerateCardNumber(brand model.CardBrand) (string, error) {
	iin, err := issuerIdentificationNumber(brand)
	if err != nil {
		return "", err
	}

	accountDigitsCount := cardNumberLength - len(iin) - 1
	accountDigits, err := randomDigits(accountDigitsCount)
	if err != nil {
		return "", err
	}

	partial := iin + accountDigits
	checkDigit := calculateLuhnCheckDigit(partial)

	return partial + checkDigit, nil
}

func GenerateCVV() (string, error) {
	return randomDigits(cardCVVLength)
}

func GenerateConfirmationCode() (string, error) {
	return randomDigits(confirmationCodeSize)
}

func GenerateCardExpiry(now time.Time) time.Time {
	expires := now.UTC().AddDate(cardExpiryYears, 0, 0)
	firstOfNextMonth := time.Date(expires.Year(), expires.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, 1, 0)
	return firstOfNextMonth.Add(-time.Nanosecond)
}

func MaskCardNumber(number string) string {
	if len(number) != cardNumberLength {
		return number
	}

	return number[:4] + "********" + number[len(number)-4:]
}

func (s *CardService) RequestCard(ctx context.Context, input *RequestCardInput) (*model.CardRequest, error) {
	if input == nil {
		return nil, errors.BadRequestErr("request body is required")
	}
	if strings.TrimSpace(input.AccountNumber) == "" {
		return nil, errors.BadRequestErr("account number is required")
	}
	authCtx := auth.GetAuthFromContext(ctx)

	account, err := s.accountRepo.FindByAccountNumber(ctx, input.AccountNumber)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	if account == nil {
		return nil, errors.NotFoundErr("account not found")
	}
	if *authCtx.ClientID != account.ClientID {
		return nil, errors.ForbiddenErr("account does not belong to authenticated client")
	}

	existingRequest, err := s.cardRequestRepo.FindLatestPendingByAccountNumber(ctx, account.AccountNumber)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	if existingRequest != nil {
		return nil, errors.ConflictErr("a pending card request already exists for this account")
	}

	if account.AccountType == model.AccountTypePersonal {
		if input.AuthorizedPerson != nil {
			return nil, errors.BadRequestErr("personal accounts cannot create cards for authorized persons")
		}

		count, err := s.cardRepo.CountNonDeactivatedByAccountNumber(ctx, account.AccountNumber)
		if err != nil {
			return nil, errors.InternalErr(err)
		}
		if count >= model.MaxPersonalCardsPerAccount {
			return nil, errors.ConflictErr("maximum number of cards reached for this account")
		}
	}

	if account.AccountType == model.AccountTypeBusiness {
		if input.AuthorizedPerson == nil {
			count, err := s.cardRepo.CountNonDeactivatedByAccountNumberAndAuthorizedPersonID(ctx, account.AccountNumber, nil)
			if err != nil {
				return nil, errors.InternalErr(err)
			}
			if count >= model.MaxBusinessCardsPerPerson {
				return nil, errors.ConflictErr("business account owner already has a card")
			}
		} else if err := validateAuthorizedPersonInput(input.AuthorizedPerson); err != nil {
			return nil, err
		}
	}

	code, err := GenerateConfirmationCode()
	if err != nil {
		return nil, errors.InternalErr(err)
	}

	request := &model.CardRequest{
		AccountNumber:       account.AccountNumber,
		ConfirmationCode:    code,
		ExpiresAt:           time.Now().UTC().Add(confirmationCodeTTL),
		Used:                false,
		ForAuthorizedPerson: input.AuthorizedPerson != nil,
	}

	if input.AuthorizedPerson != nil {
		request.AuthorizedPersonFirstName = stringPtr(strings.TrimSpace(input.AuthorizedPerson.FirstName))
		request.AuthorizedPersonLastName = stringPtr(strings.TrimSpace(input.AuthorizedPerson.LastName))
		request.AuthorizedPersonDateOfBirth = timePtr(input.AuthorizedPerson.DateOfBirth)
		request.AuthorizedPersonGender = stringPtr(strings.TrimSpace(input.AuthorizedPerson.Gender))
		request.AuthorizedPersonEmail = stringPtr(strings.TrimSpace(input.AuthorizedPerson.Email))
		request.AuthorizedPersonPhoneNumber = stringPtr(strings.TrimSpace(input.AuthorizedPerson.PhoneNumber))
		request.AuthorizedPersonAddress = stringPtr(strings.TrimSpace(input.AuthorizedPerson.Address))
	}

	if err := s.cardRequestRepo.Create(ctx, request); err != nil {
		return nil, errors.InternalErr(err)
	}

	if err := s.sendCardRequestConfirmationEmail(ctx, account.ClientID, request); err != nil {
		return nil, err
	}

	return request, nil
}

func (s *CardService) ConfirmCardRequest(ctx context.Context, accountNumber, code string) (*model.Card, error) {
	authCtx := auth.GetAuthFromContext(ctx)

	account, err := s.accountRepo.FindByAccountNumber(ctx, accountNumber)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	if account == nil {
		return nil, errors.NotFoundErr("account not found")
	}
	if *authCtx.ClientID != account.ClientID {
		return nil, errors.ForbiddenErr("account does not belong to authenticated client")
	}

	request, err := s.cardRequestRepo.FindByAccountNumberAndCode(ctx, accountNumber, code)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	if request == nil || request.Used || request.ExpiresAt.Before(time.Now().UTC()) {
		return nil, errors.BadRequestErr("invalid or expired confirmation code")
	}

	var authorizedPersonID *uint

	if account.AccountType == model.AccountTypePersonal {
		count, err := s.cardRepo.CountNonDeactivatedByAccountNumber(ctx, account.AccountNumber)
		if err != nil {
			return nil, errors.InternalErr(err)
		}
		if count >= model.MaxPersonalCardsPerAccount {
			return nil, errors.ConflictErr("maximum number of cards reached for this account")
		}
	}

	if account.AccountType == model.AccountTypeBusiness {
		if request.ForAuthorizedPerson {
			person := &model.AuthorizedPerson{
				AccountNumber: account.AccountNumber,
				FirstName:     derefString(request.AuthorizedPersonFirstName),
				LastName:      derefString(request.AuthorizedPersonLastName),
				DateOfBirth:   derefTime(request.AuthorizedPersonDateOfBirth),
				Gender:        derefString(request.AuthorizedPersonGender),
				Email:         derefString(request.AuthorizedPersonEmail),
				PhoneNumber:   derefString(request.AuthorizedPersonPhoneNumber),
				Address:       derefString(request.AuthorizedPersonAddress),
			}

			if strings.TrimSpace(person.FirstName) == "" || strings.TrimSpace(person.LastName) == "" || strings.TrimSpace(person.Email) == "" {
				return nil, errors.InternalErr(fmt.Errorf("authorized person data is incomplete"))
			}

			if err := s.authorizedPersonRepo.Create(ctx, person); err != nil {
				return nil, errors.InternalErr(err)
			}
			authorizedPersonID = &person.AuthorizedPersonID
		} else {
			count, err := s.cardRepo.CountNonDeactivatedByAccountNumberAndAuthorizedPersonID(ctx, account.AccountNumber, nil)
			if err != nil {
				return nil, errors.InternalErr(err)
			}
			if count >= model.MaxBusinessCardsPerPerson {
				return nil, errors.ConflictErr("business account owner already has a card")
			}
		}
	}

	card, err := s.createCard(ctx, account, authorizedPersonID)
	if err != nil {
		return nil, err
	}

	request.Used = true
	if err := s.cardRequestRepo.Update(ctx, request); err != nil {
		return nil, errors.InternalErr(err)
	}

	if err := s.sendCardCreatedEmail(ctx, account, card); err != nil {
		return nil, err
	}

	return card, nil
}

func (s *CardService) ListCardsForAccount(ctx context.Context, accountNumber string) (*AccountCardsResult, error) {
	account, err := s.accountRepo.FindByAccountNumber(ctx, accountNumber)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	if account == nil {
		return nil, errors.NotFoundErr("account not found")
	}

	if err := s.ensureCanAccessAccount(ctx, account); err != nil {
		return nil, err
	}

	cards, err := s.cardRepo.ListByAccountNumber(ctx, accountNumber)
	if err != nil {
		return nil, errors.InternalErr(err)
	}

	return &AccountCardsResult{
		Account: account,
		Cards:   cards,
	}, nil
}

func (s *CardService) BlockCard(ctx context.Context, cardID uint) (*model.Card, error) {
	card, account, err := s.getCardAndAccount(ctx, cardID)
	if err != nil {
		return nil, err
	}

	authCtx := auth.GetAuthFromContext(ctx)

	switch authCtx.IdentityType {
	case auth.IdentityEmployee:
	case auth.IdentityClient:
		if authCtx.ClientID == nil || *authCtx.ClientID != account.ClientID {
			return nil, errors.ForbiddenErr("card does not belong to authenticated client")
		}
		if card.AuthorizedPersonID != nil {
			return nil, errors.ForbiddenErr("client can block only their own card")
		}
	default:
		return nil, errors.ForbiddenErr("unsupported identity type")
	}

	if card.Status == model.CardStatusDeactivated {
		return nil, errors.BadRequestErr("deactivated cards cannot be blocked")
	}
	if card.Status == model.CardStatusBlocked {
		return nil, errors.BadRequestErr("card is already blocked")
	}

	card.Status = model.CardStatusBlocked
	if err := s.cardRepo.Update(ctx, card); err != nil {
		return nil, errors.InternalErr(err)
	}

	if err := s.sendCardStatusChangedEmail(ctx, account, card, "blocked"); err != nil {
		return nil, err
	}

	return card, nil
}

func (s *CardService) UnblockCard(ctx context.Context, cardID uint) (*model.Card, error) {
	card, account, err := s.getCardAndAccount(ctx, cardID)
	if err != nil {
		return nil, err
	}

	if card.Status == model.CardStatusDeactivated {
		return nil, errors.BadRequestErr("deactivated cards cannot be unblocked")
	}
	if card.Status != model.CardStatusBlocked {
		return nil, errors.BadRequestErr("only blocked cards can be unblocked")
	}

	card.Status = model.CardStatusActive
	if err := s.cardRepo.Update(ctx, card); err != nil {
		return nil, errors.InternalErr(err)
	}

	if err := s.sendCardStatusChangedEmail(ctx, account, card, "unblocked"); err != nil {
		return nil, err
	}

	return card, nil
}

func (s *CardService) DeactivateCard(ctx context.Context, cardID uint) (*model.Card, error) {
	card, account, err := s.getCardAndAccount(ctx, cardID)
	if err != nil {
		return nil, err
	}

	if card.Status == model.CardStatusDeactivated {
		return nil, errors.BadRequestErr("card is already deactivated")
	}

	card.Status = model.CardStatusDeactivated
	if err := s.cardRepo.Update(ctx, card); err != nil {
		return nil, errors.InternalErr(err)
	}

	if err := s.sendCardStatusChangedEmail(ctx, account, card, "deactivated"); err != nil {
		return nil, err
	}

	return card, nil
}

func (s *CardService) createCard(ctx context.Context, account *model.Account, authorizedPersonID *uint) (*model.Card, error) {
	brand, number, err := s.generateUniqueCardNumber(ctx)
	if err != nil {
		return nil, err
	}

	cvv, err := GenerateCVV()
	if err != nil {
		return nil, errors.InternalErr(err)
	}

	card := &model.Card{
		CardNumber:         number,
		CardType:           model.CardTypeDebit,
		CardBrand:          brand,
		Name:               fmt.Sprintf("%s %s", brand, model.CardTypeDebit),
		AccountNumber:      account.AccountNumber,
		CVV:                cvv,
		Limit:              account.MonthlyLimit,
		Status:             model.CardStatusActive,
		AuthorizedPersonID: authorizedPersonID,
		ExpiresAt:          GenerateCardExpiry(time.Now().UTC()),
	}

	if err := s.cardRepo.Create(ctx, card); err != nil {
		return nil, errors.InternalErr(err)
	}

	return card, nil
}

func (s *CardService) generateUniqueCardNumber(ctx context.Context) (model.CardBrand, string, error) {
	for {
		brand, err := GenerateCardBrand()
		if err != nil {
			return "", "", errors.InternalErr(err)
		}

		number, err := GenerateCardNumber(brand)
		if err != nil {
			return "", "", errors.InternalErr(err)
		}

		exists, err := s.cardRepo.CardNumberExists(ctx, number)
		if err != nil {
			return "", "", errors.InternalErr(err)
		}
		if !exists {
			return brand, number, nil
		}
	}
}

func (s *CardService) getCardAndAccount(ctx context.Context, cardID uint) (*model.Card, *model.Account, error) {
	card, err := s.cardRepo.FindByID(ctx, cardID)
	if err != nil {
		return nil, nil, errors.InternalErr(err)
	}
	if card == nil {
		return nil, nil, errors.NotFoundErr("card not found")
	}

	account, err := s.accountRepo.FindByAccountNumber(ctx, card.AccountNumber)
	if err != nil {
		return nil, nil, errors.InternalErr(err)
	}
	if account == nil {
		return nil, nil, errors.NotFoundErr("account not found")
	}

	return card, account, nil
}

func (s *CardService) ensureCanAccessAccount(ctx context.Context, account *model.Account) error {
	authCtx := auth.GetAuthFromContext(ctx)

	switch authCtx.IdentityType {
	case auth.IdentityEmployee:
		return nil
	case auth.IdentityClient:
		if authCtx.ClientID == nil {
			return errors.UnauthorizedErr("not authenticated")
		}
		if *authCtx.ClientID != account.ClientID {
			return errors.ForbiddenErr("account does not belong to authenticated client")
		}
		return nil
	default:
		return errors.ForbiddenErr("unsupported identity type")
	}
}

func validateAuthorizedPersonInput(person *AuthorizedPersonInput) error {
	if person == nil {
		return nil
	}
	if strings.TrimSpace(person.FirstName) == "" {
		return errors.BadRequestErr("authorized person first name is required")
	}
	if strings.TrimSpace(person.LastName) == "" {
		return errors.BadRequestErr("authorized person last name is required")
	}
	if strings.TrimSpace(person.Email) == "" {
		return errors.BadRequestErr("authorized person email is required")
	}

	return nil
}

func (s *CardService) sendCardRequestConfirmationEmail(ctx context.Context, clientID uint, request *model.CardRequest) error {
	email, fullName, err := s.getClientContact(ctx, clientID)
	if err != nil {
		return err
	}

	body := fmt.Sprintf(
		"Hello %s,\n\nYour card request for account %s has been received.\nUse this confirmation code to complete the request: %s\n\nThis code expires at %s UTC.",
		defaultContactName(fullName),
		request.AccountNumber,
		request.ConfirmationCode,
		request.ExpiresAt.UTC().Format(time.RFC3339),
	)

	if err := s.mailer.Send(email, "Confirm your card request", body); err != nil {
		return errors.ServiceUnavailableErr(err)
	}

	return nil
}

func (s *CardService) sendCardCreatedEmail(ctx context.Context, account *model.Account, card *model.Card) error {
	email, fullName, err := s.getClientContact(ctx, account.ClientID)
	if err != nil {
		return err
	}

	body := fmt.Sprintf(
		"Hello %s,\n\nA new card for account %s has been created successfully.\nCard number: %s\nStatus: %s\nExpires at: %s UTC.",
		defaultContactName(fullName),
		account.AccountNumber,
		MaskCardNumber(card.CardNumber),
		card.Status,
		card.ExpiresAt.UTC().Format(time.RFC3339),
	)

	if err := s.mailer.Send(email, "Card created successfully", body); err != nil {
		return errors.ServiceUnavailableErr(err)
	}

	return nil
}

func (s *CardService) sendCardStatusChangedEmail(ctx context.Context, account *model.Account, card *model.Card, status string) error {
	recipients, err := s.getCardStatusNotificationRecipients(ctx, account, card)
	if err != nil {
		return err
	}

	body := fmt.Sprintf(
		"Card %s for account %s has been %s.\nCurrent status: %s.",
		MaskCardNumber(card.CardNumber),
		account.AccountNumber,
		status,
		card.Status,
	)

	for _, email := range recipients {
		if err := s.mailer.Send(email, "Card status changed", body); err != nil {
			return errors.ServiceUnavailableErr(err)
		}
	}

	return nil
}

func (s *CardService) getCardStatusNotificationRecipients(ctx context.Context, account *model.Account, card *model.Card) ([]string, error) {
	ownerEmail, _, err := s.getClientContact(ctx, account.ClientID)
	if err != nil {
		return nil, err
	}

	recipients := []string{ownerEmail}

	if account.AccountType == model.AccountTypeBusiness && card.AuthorizedPersonID != nil {
		person, err := s.authorizedPersonRepo.FindByID(ctx, *card.AuthorizedPersonID)
		if err != nil {
			return nil, errors.InternalErr(err)
		}
		if person == nil {
			return nil, errors.InternalErr(fmt.Errorf("authorized person not found"))
		}

		personEmail := strings.TrimSpace(person.Email)
		if personEmail != "" && !containsString(recipients, personEmail) {
			recipients = append(recipients, personEmail)
		}
	}

	return recipients, nil
}

func (s *CardService) getClientContact(ctx context.Context, clientID uint) (string, string, error) {
	response, err := s.userClient.GetClientByID(ctx, clientID)
	if err != nil {
		return "", "", errors.ServiceUnavailableErr(err)
	}
	if response == nil {
		return "", "", errors.InternalErr(fmt.Errorf("client not found in user service"))
	}

	email := strings.TrimSpace(response.GetEmail())
	if email == "" {
		return "", "", errors.InternalErr(fmt.Errorf("client email is missing"))
	}

	return email, strings.TrimSpace(response.GetFullName()), nil
}

func defaultContactName(fullName string) string {
	if strings.TrimSpace(fullName) == "" {
		return "client"
	}

	return fullName
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(target)) {
			return true
		}
	}

	return false
}

func issuerIdentificationNumber(brand model.CardBrand) (string, error) {
	switch brand {
	case model.CardBrandVisa:
		return visaIIN, nil
	case model.CardBrandMasterCard:
		return masterCardIIN, nil
	case model.CardBrandDinaCard:
		return dinaCardIIN, nil
	default:
		return "", fmt.Errorf("unsupported card brand: %s", brand)
	}
}

func randomDigits(length int) (string, error) {
	bytes := make([]byte, length)
	for i := 0; i < length; i++ {
		value, err := randomInt(10)
		if err != nil {
			return "", err
		}
		bytes[i] = byte('0' + value)
	}

	return string(bytes), nil
}

func randomInt(max int) (int, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0, err
	}

	return int(n.Int64()), nil
}

func calculateLuhnCheckDigit(numberWithoutCheckDigit string) string {
	sum := 0
	double := true

	for i := len(numberWithoutCheckDigit) - 1; i >= 0; i-- {
		digit := int(numberWithoutCheckDigit[i] - '0')
		if double {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}
		sum += digit
		double = !double
	}

	checkDigit := (10 - (sum % 10)) % 10
	return string(rune('0' + checkDigit))
}

func stringPtr(value string) *string {
	return &value
}

func timePtr(value time.Time) *time.Time {
	return &value
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}

	return *value
}

func derefTime(value *time.Time) time.Time {
	if value == nil {
		return time.Time{}
	}

	return *value
}
