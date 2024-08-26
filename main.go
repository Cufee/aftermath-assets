package main

import (
	"os"
	"path/filepath"

	"github.com/alexflint/go-arg"

	_ "github.com/joho/godotenv/autoload"
)

var args struct {
	DumpPath   string `arg:"positional,required" help:"path to the depot dump directory" placeholder:"<dump_path>"`
	AssetsPath string `arg:"positional,required" help:"output directory for parsed assets" placeholder:"<assets_path>"`

	Download           bool   `help:"download the steam app depot"`
	DownloaderPath     string `arg:"--binary,env:DOWNLOADER_CMD_PATH" help:"path to steam depot downloader binary" placeholder:"<path>"`
	AppID              string `arg:"--app,env:DOWNLOADER_APP_ID" help:"steam app id for depot" placeholder:"<app_id>"`
	DepotID            string `arg:"--depot,env:DOWNLOADER_DEPOT_ID" help:"steam depot if to download" placeholder:"<depot_id>"`
	SteamUsername      string `arg:"--username,env:DOWNLOADER_STEAM_USERNAME" help:"steam account username" placeholder:"<username>"`
	SteamPassword      string `arg:"--password,env:DOWNLOADER_STEAM_PASSWORD" help:"steam account password" placeholder:"<password>"`
	SteamAuthCode      string `arg:"--auth-code,env:DOWNLOADER_STEAM_AUTH_CODE" help:"steam one time auth code" placeholder:"<code>"`
	DownloaderFileList string `arg:"--file-list,env:DOWNLOADER_FILE_LIST" help:"path to filelist.txt" placeholder:"<path>"`

	Decrypt     bool   `help:"decrypt downloaded files"`
	DecryptPath string `arg:"--decrypt-path,env:DECRYPT_DIR_PATH" help:"path to a directory where decrypted files will be stored" placeholder:"<decrypted_path>"`

	Parse bool `help:"parse decrypted files into asset strings"`

	WargamingAppID string `arg:"--app-id,env:WARGAMING_APP_ID" help:"wargaming application id for api requests" placeholder:"<key>"`

	EmailEnabled bool `arg:"--mail" help:"enabled parsing steam auth code from email"`
	emailConfig
}

func main() {
	arg.MustParse(&args)

	cdn := NewCDNClient(args.WargamingAppID)

	if args.Download {
		var client *emailClient
		if args.EmailEnabled {
			c, err := newEmailClient(args.emailConfig)
			if err != nil {
				panic(err)
			}
			client = c
		}

		err := downloadAssetsFromSteam(client)
		if err != nil {
			panic(err)
		}
	}

	if args.Decrypt {
		// Decrypt downloaded files
		stringsPath := filepath.Join(args.DumpPath, "Data")
		dir, err := os.ReadDir(stringsPath)
		if err != nil {
			panic(err)
		}
		err = decryptDir(stringsPath, dir, args.DecryptPath)
		if err != nil {
			panic(err)
		}

		err = mergeMissingStrings(cdn, args.DecryptPath)
		if err != nil {
			panic(err)
		}
	}

	if args.Parse {
		glossary, err := cdn.Vehicles(cdnLanguages...)
		if err != nil {
			panic(err)
		}

		err = os.MkdirAll(args.DecryptPath, os.ModeDir)
		if err != nil {
			panic(err)
		}

		// Init parsing functions
		maps := newMapParser()
		version := newVersionParser()
		vehicles := newVehiclesParser(glossary)
		battleTypes := newBattleTypeParser()

		// Due to how the parsing code is written, we will need to loop over the files twice
		// first loop parses yaml/xml files to extract identifier
		// second loop will parse strings yaml files to create localized dicts
		{
			parser, err := newParser(args.DecryptPath, maps.Maps(), vehicles.Items(), battleTypes, version)
			if err != nil {
				panic(err)
			}
			if err := parser.Parse(); err != nil {
				panic(err)
			}
		}
		{
			parser, err := newParser(args.DecryptPath, maps.Strings(), vehicles.Strings())
			if err != nil {
				panic(err)
			}
			if err := parser.Parse(); err != nil {
				panic(err)
			}
		}

		err = maps.Export(filepath.Join(args.AssetsPath, "maps.json"))
		if err != nil {
			panic(err)
		}
		err = vehicles.Export(filepath.Join(args.AssetsPath, "vehicles.json"))
		if err != nil {
			panic(err)
		}
		err = version.Export(filepath.Join(args.AssetsPath, "metadata.json"))
		if err != nil {
			panic(err)
		}
		err = battleTypes.Export(filepath.Join(args.AssetsPath, "game_modes.json"))
		if err != nil {
			panic(err)
		}
	}
}
