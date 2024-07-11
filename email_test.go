package main

import (
	"os"
	"testing"
	"time"

	"github.com/matryer/is"

	_ "github.com/joho/godotenv/autoload"
)

func TestEmailClient(t *testing.T) {
	is := is.New(t)

	_, err := newEmailClient(emailConfig{Host: "", User: "", Password: ""})
	is.True(err != nil)

	cfg := emailConfig{
		Host:     os.Getenv("EMAIL_HOST"),
		User:     os.Getenv("EMAIL_USER"),
		Password: os.Getenv("EMAIL_PASSWORD"),
	}

	client, err := newEmailClient(cfg)
	is.NoErr(err)

	emails, err := client.GetSince(time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC))
	is.NoErr(err)

	is.True(len(emails) > 0)

	addr := os.Getenv("EMAIL_TEST_SENDER")

	var code string
	for _, email := range emails {
		c, ok := findSteamCode(email, addr)
		if ok {
			code = c
			break
		}
	}

	is.True(code != "")
}
