package service

import (
	"banking-service/internal/model"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGenerateCardBrand(t *testing.T) {
	t.Parallel()

	brand, err := GenerateCardBrand()

	require.NoError(t, err)
	require.Contains(t, []model.CardBrand{
		model.CardBrandVisa,
		model.CardBrandMasterCard,
		model.CardBrandDinaCard,
	}, brand)
}

func TestGenerateCardNumber(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		brand        model.CardBrand
		prefixChecks []string
	}{
		{
			name:         "visa",
			brand:        model.CardBrandVisa,
			prefixChecks: []string{"4"},
		},
		{
			name:         "mastercard",
			brand:        model.CardBrandMasterCard,
			prefixChecks: []string{"53"},
		},
		{
			name:         "dinacard",
			brand:        model.CardBrandDinaCard,
			prefixChecks: []string{"9891"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			number, err := GenerateCardNumber(tt.brand)

			require.NoError(t, err)
			require.Len(t, number, 16)
			require.True(t, hasAnyPrefix(number, tt.prefixChecks))
			require.True(t, isValidLuhn(number))
		})
	}
}

func TestGenerateCardNumberUnsupportedBrand(t *testing.T) {
	t.Parallel()

	_, err := GenerateCardNumber(model.CardBrand("Unknown"))

	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported card brand")
}

func TestGenerateCVV(t *testing.T) {
	t.Parallel()

	cvv, err := GenerateCVV()

	require.NoError(t, err)
	require.Len(t, cvv, 3)
	require.True(t, digitsOnly(cvv))
}

func TestGenerateConfirmationCode(t *testing.T) {
	t.Parallel()

	code, err := GenerateConfirmationCode()

	require.NoError(t, err)
	require.Len(t, code, 6)
	require.True(t, digitsOnly(code))
}

func TestGenerateCardExpiry(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.March, 17, 12, 0, 0, 0, time.UTC)

	expiresAt := GenerateCardExpiry(now)

	require.Equal(t, 2030, expiresAt.Year())
	require.Equal(t, time.March, expiresAt.Month())
	require.Equal(t, 31, expiresAt.Day())
}

func TestMaskCardNumber(t *testing.T) {
	t.Parallel()

	masked := MaskCardNumber("5798123412345571")

	require.Equal(t, "5798********5571", masked)
}

func TestMaskCardNumberInvalidLengthReturnsOriginal(t *testing.T) {
	t.Parallel()

	require.Equal(t, "12345", MaskCardNumber("12345"))
}

func hasAnyPrefix(value string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if len(value) >= len(prefix) && value[:len(prefix)] == prefix {
			return true
		}
	}

	return false
}

func digitsOnly(value string) bool {
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return false
		}
	}

	return true
}

func isValidLuhn(number string) bool {
	sum := 0
	double := false

	for i := len(number) - 1; i >= 0; i-- {
		digit := int(number[i] - '0')
		if double {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}
		sum += digit
		double = !double
	}

	return sum%10 == 0
}