package tgclient

import (
	"fmt"
	"strings"
	"unicode"
)

func normalizePhone(phone string) string {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return phone
	}

	var digits strings.Builder
	for _, r := range phone {
		if unicode.IsDigit(r) {
			digits.WriteRune(r)
		}
	}
	d := digits.String()
	if d == "" {
		return phone
	}

	// Russia: 8XXXXXXXXXX or 7XXXXXXXXXX -> +7XXXXXXXXXX
	switch {
	case len(d) == 11 && d[0] == '8':
		return "+7" + d[1:]
	case len(d) == 11 && d[0] == '7':
		return "+" + d
	case strings.HasPrefix(phone, "+") && len(d) == 11 && d[0] == '8':
		return "+7" + d[1:]
	case strings.HasPrefix(phone, "+") && len(d) >= 10:
		return "+" + d
	default:
		return "+" + d
	}
}

func validatePhone(phone string) error {
	phone = normalizePhone(phone)
	if phone == "" {
		return fmt.Errorf("phone number is required")
	}
	if !strings.HasPrefix(phone, "+") {
		return fmt.Errorf("phone must be in international format, e.g. +79991234567")
	}

	digits := phone[1:]
	for _, ch := range digits {
		if ch < '0' || ch > '9' {
			return fmt.Errorf("phone must contain only digits after +")
		}
	}

	// Russia (+7): 7 + 10 digits, e.g. +79991234567
	if strings.HasPrefix(phone, "+7") {
		if len(digits) != 11 || digits[0] != '7' {
			return fmt.Errorf("russian phone must be +7 followed by 10 digits, e.g. +79991234567 (got %s)", phone)
		}
		return nil
	}

	if len(digits) < 8 || len(digits) > 15 {
		return fmt.Errorf("phone length looks invalid: %s", phone)
	}
	return nil
}
