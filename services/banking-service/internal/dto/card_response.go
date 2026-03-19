package dto

import (
	"banking-service/internal/model"
	"time"
)

type CardResponse struct {
	ID                 uint      `json:"id"`
	AccountNumber      string    `json:"account_number"`
	AccountName        string    `json:"account_name,omitempty"`
	MaskedCardNumber   string    `json:"card_number"`
	CardType           string    `json:"card_type"`
	CardBrand          string    `json:"card_brand"`
	Name               string    `json:"name"`
	ExpiresAt          time.Time `json:"expires_at"`
	Limit              float64   `json:"limit"`
	Status             string    `json:"status"`
	AuthorizedPersonID *uint     `json:"authorized_person_id,omitempty"`
}

type AccountCardsResponse struct {
	AccountNumber string         `json:"account_number"`
	AccountName   string         `json:"account_name"`
	Cards         []CardResponse `json:"cards"`
}

func ToCardResponse(card *model.Card, accountName string) CardResponse {
	return CardResponse{
		ID:                 card.CardID,
		AccountNumber:      card.AccountNumber,
		AccountName:        accountName,
		MaskedCardNumber:   maskCardNumber(card.CardNumber),
		CardType:           string(card.CardType),
		CardBrand:          string(card.CardBrand),
		Name:               card.Name,
		ExpiresAt:          card.ExpiresAt,
		Limit:              card.Limit,
		Status:             string(card.Status),
		AuthorizedPersonID: card.AuthorizedPersonID,
	}
}

func ToAccountCardsResponse(account *model.Account, cards []model.Card) AccountCardsResponse {
	responseCards := make([]CardResponse, len(cards))
	for i, card := range cards {
		responseCards[i] = ToCardResponse(&card, account.Name)
	}

	return AccountCardsResponse{
		AccountNumber: account.AccountNumber,
		AccountName:   account.Name,
		Cards:         responseCards,
	}
}

func maskCardNumber(number string) string {
	if len(number) != 16 {
		return number
	}

	return number[:4] + "********" + number[len(number)-4:]
}