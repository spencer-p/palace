package main

import (
	"bytes"
	"errors"
	"os"

	"github.com/spencer-p/palace/pkg/auth"
)

var theOnePassword = auth.MustDecodeBase64([]byte(os.Getenv("MY_PASSWORD")))

type fakeUsersDB struct{}

func (db fakeUsersDB) ValidatePassword(username string, password []byte) error {
	if username != "spencer" {
		return errors.New("no account")
	}
	if !bytes.Equal(password, theOnePassword) {
		return errors.New("password incorrect")
	}
	return nil
}
