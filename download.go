package main

import (
	"bufio"
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
	cmd.Stderr = os.Stderr

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	defer stdinPipe.Close()

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	defer stdoutPipe.Close()

	err = cmd.Start()
	if err != nil {
		return errors.Wrap(err, "cmd#Start")
	}
	stdoutTee := io.TeeReader(stdoutPipe, os.Stdout)

	if args.SteamAuthCode != "" {
		// input the auth code if it was provided
		go func() {
			defer stdinPipe.Close()
			io.WriteString(stdinPipe, args.SteamAuthCode)
		}()
	} else if email != nil {
		// start a new scanner to check if the download started
		scanner := bufio.NewScanner(stdoutTee)
		started := make(chan struct{})
		go func() {
			// this scanner will run until the app exits, but that's barely an inconvenience
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if strings.Contains(line, "Got AppInfo for") {
					started <- struct{}{}
				}
			}
		}()

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()

		select {
		case <-ctx.Done():
			delay := time.Second * 30
			log.Printf("Download failed to start, checking email for auth code in %.0f seconds\n", delay.Seconds())
			time.Sleep(delay)

			code, err := email.GetSteamCode(time.Now().Add(time.Minute * -1))
			if err != nil {
				return errors.Wrap(err, "failed to get steam auth code from email")
			}

			go func() {
				defer stdinPipe.Close()
				io.WriteString(stdinPipe, code)
			}()

			break

		case <-started:
			log.Println("Downloader started download successfully")
			cancel()
		}
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
