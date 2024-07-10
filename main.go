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
	DownloaderPath     string `arg:"--binary,env:DOWNLOADER_CMD_PATH" default:"./downloader/bin" help:"path to steam depot downloader binary" placeholder:"<path>"`
	AppID              string `arg:"--app,env:DOWNLOADER_APP_ID" help:"steam app id for depot" placeholder:"<app_id>"`
	DepotID            string `arg:"--depot,env:DOWNLOADER_DEPOT_ID" help:"steam depot if to download" placeholder:"<depot_id>"`
	SteamUsername      string `arg:"--username,env:DOWNLOADER_STEAM_USERNAME" help:"steam account username" placeholder:"<username>"`
	SteamPassword      string `arg:"--password,env:DOWNLOADER_STEAM_PASSWORD" help:"steam account password" placeholder:"<password>"`
	DownloaderFileList string `arg:"--file-list" default:"./filelist.txt" placeholder:"<path>"`

	Decrypt     bool   `help:"decrypt downloaded files"`
	DecryptPath string `arg:"--decrypt-path,env:DECRYPT_DIR_PATH" default:"./tmp/decrypted" help:"path to a directory where decrypted files will be stored" placeholder:"<decrypted_path>"`

	Parse bool `help:"parse decrypted files into asset strings"`
}

func main() {
	arg.MustParse(&args)

	if args.Download {
		err := downloadAssetsFromSteam()
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
		err = os.MkdirAll(filepath.Dir(args.AssetsPath), os.ModePerm)
		if err != nil {
			panic(err)
		}
		err = decryptDir(stringsPath, dir, args.DecryptPath)
		if err != nil {
			panic(err)
		}
	}

	// Init parsing functions
	maps := newMapParser()
	vehicles := newVehiclesParser()

	// Due to how the parsing code is written, we will need to loop over the files twice
	// first loop parses yaml/xml files to extract identifier
	// second loop will parse strings yaml files to create localized dicts
	{
		parser, err := newParser(args.DecryptPath, maps.Maps(), vehicles.Items())
		if err != nil {
			panic(err)
		}
		if err := parser.Parse(); err != nil {
			panic(err)
		}
	}
	{
		parser, err := newParser(args.DecryptPath, maps.Strings(args.AssetsPath), vehicles.Strings(args.AssetsPath))
		if err != nil {
			panic(err)
		}
		if err := parser.Parse(); err != nil {
			panic(err)
		}
	}

}
