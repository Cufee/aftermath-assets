package main

import (
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/jprobinson/eazye"
	"github.com/pkg/errors"
)

type emailConfig struct {
	Host     string `arg:"--mail-host,env:EMAIL_HOST" help:"email host for retrieving auth keys" placeholder:"<host>"`
	Password string `arg:"--mail-pass,env:EMAIL_PASSWORD" help:"email username" placeholder:"<password>"`
	User     string `arg:"--mail-user,env:EMAIL_USER" help:"email username" placeholder:"<username>"`
}

func (c emailConfig) Info() eazye.MailboxInfo {
	return eazye.MailboxInfo{
		TLS:    true,
		Host:   c.Host,
		User:   c.User,
		Pwd:    c.Password,
		Folder: "Inbox",
	}
}

type emailClient struct {
	config emailConfig
}

func newEmailClient(cfg emailConfig) (*emailClient, error) {
	if cfg.Host == "" || cfg.Password == "" || cfg.User == "" {
		return nil, errors.New("missing host, password or user")
	}
	return &emailClient{cfg}, nil
}

func (c emailClient) GetSince(since time.Time) ([]eazye.Email, error) {
	return eazye.GetSince(c.config.Info(), since, false, false)
}

func (c emailClient) GetSteamCode(after time.Time) (string, error) {
	emails, err := c.GetSince(after)
	if err != nil {
		return "", err
	}

	slices.SortFunc(emails, func(a, b eazye.Email) int {
		return b.InternalDate.Compare(a.InternalDate)
	})

	for _, email := range emails {
		if email.From.Address != "noreply@steampowered.com" {
			continue
		}
		code, ok := findSteamCode(string(email.Text))
		if ok {
			return code, nil
		}
	}
	return "", errors.New("steam login code not found")
}

var steamCodeRegex = regexp.MustCompile(`(?i)Login\s*Code\s*\n*\s*([A-Z0-9]{5,7})`)

func findSteamCode(text string) (string, bool) {
	matches := steamCodeRegex.FindStringSubmatch(text)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1]), true
	}
	return "", false
}
