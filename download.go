package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
)

func downloadAssetsFromSteam() error {
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
	dargs = append(dargs, "-filelist")
	dargs = append(dargs, "filelist.txt")
	dargs = append(dargs, "-dir")
	dargs = append(dargs, args.DumpPath)

	cmd := exec.Command(args.DownloaderPath, dargs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if args.SteamAuthCode != "" {
		stdin, err := cmd.StdinPipe()
		if err != nil {
			return err
		}
		go func() {
			defer stdin.Close()
			io.WriteString(stdin, args.SteamAuthCode)
		}()
	}

	err := cmd.Run()
	if err != nil {
		return err
	}

	if cmd.ProcessState.ExitCode() != 0 {
		return fmt.Errorf("bad exit code from downloader: %d", cmd.ProcessState.ExitCode())
	}
	return nil
}
