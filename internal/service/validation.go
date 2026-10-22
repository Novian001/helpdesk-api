package service

import (
	"errors"
	"net/mail"
	"strings"

	"github.com/novian001/helpdesk-api/internal/apperr"
)

// validateEmail checks that the address is syntactically valid.
func validateEmail(email string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return apperr.Unprocessable("email is required").WithDetails(map[string]string{"email": "must not be empty"})
	}
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return apperr.Unprocessable("email is not valid").WithDetails(map[string]string{"email": "must be a valid email address"})
	}
	return nil
}

// validatePassword enforces the minimum password policy.
func validatePassword(password string) error {
	if len(password) < 8 {
		return apperr.Unprocessable("password is too short").WithDetails(map[string]string{"password": "must be at least 8 characters"})
	}
	if len(password) > 72 {
		// Argon2 accepts longer inputs, but capping keeps the policy explicit.
		return apperr.Unprocessable("password is too long").WithDetails(map[string]string{"password": "must be at most 72 characters"})
	}
	return nil
}

// fieldDetail extracts the detail message of a field from a validation error.
func fieldDetail(err error, field string) string {
	var appErr *apperr.Error
	if errors.As(err, &appErr) {
		if detail, ok := appErr.Details[field]; ok {
			return detail
		}
	}
	return "is invalid"
}

// dummyHash is a valid Argon2id hash of a random value. It is used to keep the
// login timing similar for unknown emails.
const dummyHash = "$argon2id$v=19$m=65536,t=3,p=4$Y2FuYXJ5c2FsdHNhbHRzYWx0$Y2FuYXJ5c2FsdHNhbHRzYWx0c2FsdHNhbHRzYWx0c2E"
