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

	code, err := client.GetSteamCode(time.Now().Add(time.Hour * -1))
	is.NoErr(err)
	is.True(code != "")
	println(code)
}

func TestFindSteamCode(t *testing.T) {
	is := is.New(t)

	text := `
Some random text before

Login Code

T3H86

If this wasn't you

Some random text after
`

	code, ok := findSteamCode(text, "")
	is.True(ok)
	is.True(code != "")

	println(code)
}
