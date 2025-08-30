package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/creack/pty"
	"github.com/pkg/errors"
	"golang.org/x/text/language"
)

func downloadAssetsFromSteam(email *emailClient) error {
	var dargs []string
	dargs = append(dargs, "-app")
	dargs = append(dargs, args.AppID)
	dargs = append(dargs, "-depot")
	dargs = append(dargs, args.DepotID)
	dargs = append(dargs, "-username")
	dargs = append(dargs, args.SteamUsername)
	dargs = append(dargs, "-password")
	dargs = append(dargs, args.SteamPassword)
	dargs = append(dargs, "-remember-password")
	if args.DownloaderFileList != "" {
		dargs = append(dargs, "-filelist")
		dargs = append(dargs, args.DownloaderFileList)
	}
	dargs = append(dargs, "-dir")
	dargs = append(dargs, args.DumpPath)

	cmd := exec.Command(args.DownloaderPath, dargs...)
	cmd.WaitDelay = time.Minute * 15

	startedAt := time.Now().Add(time.Second * -1)

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return errors.Wrap(err, "cmd#Start")
	}
	defer ptmx.Close()

	input := func(text string) {
		_, err := ptmx.Write([]byte(text + "\n"))
		if err != nil {
			log.Print("error writing to pty", "error", err)
		}
	}

	steamGuardRequired := make(chan struct{})
	downloadStarted := make(chan struct{})
	defer func() {
		close(steamGuardRequired)
		close(downloadStarted)
	}()

	var outputBuffer bytes.Buffer
	combinedWriter := io.MultiWriter(&outputBuffer, os.Stderr)
	go func() {
		_, err := io.Copy(combinedWriter, ptmx)
		if err != nil {
			log.Print("io.Copy from ptmx finished", err)
		}
	}()

	ticker := time.NewTicker(time.Millisecond * 200)
	defer ticker.Stop()

	go func() {
		for range ticker.C {
			fullOut := outputBuffer.String()
			if strings.Contains(fullOut, "STEAM GUARD! Please enter the auth code sent to the email") {
				select {
				case steamGuardRequired <- struct{}{}:
					println() // insert a newline
				default:
				}
			}
			if strings.Contains(fullOut, "Got AppInfo for") {
				select {
				case downloadStarted <- struct{}{}:
				default:
				}
			}
		}
	}()

	startCtx, cancelStart := context.WithTimeout(context.Background(), time.Minute)
	defer cancelStart()

	select {
	case <-steamGuardRequired:
		ticker := time.NewTicker(time.Second * 5)
		defer ticker.Stop()

		var counter int
		for range ticker.C {
			counter++
			if counter > 10 {
				return errors.New("failed to find a steam guard code")
			}

			log.Print("Checking email for Steam Guard code", counter)
			code, err := email.GetSteamCode(startedAt)
			if errors.Is(err, ErrCodeNotFound) {
				continue
			}
			if err != nil {
				return errors.Wrap(err, "failed to get steam auth code from email")
			}

			log.Print("Entering Steam Guard code from email")
			input(code)
			break
		}

	case <-downloadStarted:
		log.Print("Download started")

	case <-startCtx.Done():
		log.Print("Failed to start a download, timeout")
	}

	err = cmd.Wait()
	if err != nil {
		return err
	}
	return nil
}

func mergeMissingStrings(client *wargamingCDNClient, dir string) error {
	stringFiles, err := os.ReadDir(filepath.Join(dir, "Strings"))
	if err != nil {
		return err
	}

	missingStrings, err := client.MissingStrings("en", "ru", "pl", "de", "fr", "es", "tr", "cs", "th", "vi", "ko")
	if err != nil {
		return err
	}

	for _, file := range stringFiles {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".yaml") {
			continue
		}

		fileName := strings.TrimSuffix(file.Name(), ".yaml")
		tag, err := language.Parse(fileName)
		if err != nil {
			return err
		}

		stringsFile, err := os.ReadFile(filepath.Join(dir, "Strings", file.Name()))
		if err != nil {
			return err
		}

		current, err := decodeYAML[map[string]any](bytes.NewBuffer(stringsFile))
		if err != nil {
			return err
		}

		for key, value := range missingStrings[tag] {
			if _, ok := current[key]; ok {
				continue
			}
			current[key] = value
		}

		buf, err := json.Marshal(current)
		if err != nil {
			return err
		}

		err = os.WriteFile(filepath.Join(dir, "Strings", fileName+".json"), buf, os.ModePerm)
		if err != nil {
			return err
		}
	}

	return nil
}
