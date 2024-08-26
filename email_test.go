package main

import (
	"testing"

	"github.com/matryer/is"

	_ "github.com/joho/godotenv/autoload"
)

func TestEmailClient(t *testing.T) {
	is := is.New(t)

	_, err := newEmailClient(emailConfig{Host: "", User: "", Password: ""})
	is.True(err != nil)
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

	code, ok := findSteamCode(text)
	is.True(ok)
	is.True(code != "")

	println(code)
}
