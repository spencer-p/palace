package auth

import (
	"fmt"
	"testing"
	"time"
)

func TestAuthenticator(t *testing.T) {
	db := fakeDB(func(user, password string) error {
		if user+"|"+password != "admin|password" {
			return fmt.Errorf("invalid pasword")
		}
		return nil
	})
	zeroKey := make([]byte, 16)
	auth, err := New(db, zeroKey, zeroKey)
	if err != nil {
		t.Errorf("failed to set up auth: %v", err)
	}
	auth.now = func() time.Time { return time.Time{} }

	authData, err := auth.Token("admin", "password")
	if err != nil {
		t.Errorf("failed to get new auth data: %v", err)
		return
	}

	err = auth.Validate(authData)
	if err != nil {
		t.Errorf("failed to validate auth data: %v", err)
	}

	authData[len(authData)-1]++ // Break the signature.
	err = auth.Validate(authData)
	if err == nil {
		t.Errorf("validated auth data, but wanted error")
	}
}

type fakeDB func(user, password string) error

func (f fakeDB) ValidatePassword(user, password string) error {
	return f(user, password)
}
