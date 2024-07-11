package main

import (
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

	for _, email := range emails {
		code, ok := findSteamCode(email, "noreply@steampowered.com")
		if ok {
			return code, nil
		}
	}
	return "", errors.New("steam login code not found")
}

func findSteamCode(email eazye.Email, requireAddress string) (string, bool) {
	if email.From.Address != requireAddress {
		return "", false
	}

	var validLines []string
	lines := strings.Split(string(email.Text), "\n")
	for _, line := range lines {
		if l := strings.TrimSpace(line); l != "" {
			validLines = append(validLines, l)
		}
	}

	for i, line := range validLines {
		if i+1 < len(lines) && line == "Login Code" {
			code := lines[i+1]
			if code != "" {
				return code, true
			}
			return "", false
		}
	}
	return "", false
}
