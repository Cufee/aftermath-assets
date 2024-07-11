package main

import (
	"bufio"
	"context"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/pkg/errors"
)

func downloadAssetsFromSteam(email *emailClient) error {
	startTime := time.Now()

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
	stdoutTee := io.TeeReader(stdoutPipe, os.Stdout)

	err = cmd.Start()
	if err != nil {
		return errors.Wrap(err, "cmd#Start")
	}

	if args.SteamAuthCode != "" {
		// input the auth code if it was provided
		io.WriteString(stdinPipe, args.SteamAuthCode)
	} else if email != nil {
		// start a new scanner to check if the download started
		scanner := bufio.NewScanner(stdoutTee)

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()

		started := make(chan struct{})
		go func() {
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if strings.Contains(line, "Got AppInfo for") {
					log.Println("Downloader started download successfully")
					started <- struct{}{}
				}
			}
		}()

		select {
		case <-ctx.Done():
			println("\ndownload failed to start, checking email for auth code in 10 seconds")
			err = cmd.Process.Kill()
			if err != nil {
				return errors.Wrap(err, "failed to kill a running downloader process")
			}

			time.Sleep(time.Second * 10)
			code, err := email.GetSteamCode(startTime)
			if err != nil {
				return errors.Wrap(err, "failed to get steam auth code from email")
			}
			args.SteamAuthCode = code
			return downloadAssetsFromSteam(nil)

		case <-started:
			println("\ndownload started")
		}
	}

	err = cmd.Wait()
	if err != nil {
		return err
	}
	return nil
}
