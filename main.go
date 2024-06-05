package main

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"

	"encoding/binary"
	"errors"
	"hash/crc32"

	_ "github.com/joho/godotenv/autoload"
	"github.com/pierrec/lz4/v4"
)

const baseDir = "./downloaded/Data/Strings"

type parsedData struct {
	data map[string]map[string]string
}

type parsedKey string

func main() {
	err := downloadAssetsFromSteam(os.Getenv("DOWNLOADER_CMD_PATH"))
	if err != nil {
		panic(err)
	}

	dir, err := os.ReadDir(baseDir)
	if err != nil {
		panic(err)
	}

	var wg sync.WaitGroup

	for _, entry := range dir {
		wg.Add(1)
		go func(entry fs.DirEntry) {
			defer wg.Done()
			if entry.IsDir() || !strings.Contains(entry.Name(), ".yaml") {
				return
			}
			log.Println("decrypting and parsing", entry.Name())

			raw, err := os.ReadFile(filepath.Join(baseDir, entry.Name()))
			if err != nil {
				panic(err)
			}

			lang := strings.Split(entry.Name(), ".")[0]
			locale, err := language.Parse(lang)
			if err != nil {
				panic(err)
			}

			path := filepath.Join("./assets/", locale.String(), "game_strings.yaml")
			err = parseAndSaveFile(raw, path)
			if err != nil {
				panic(err)
			}

			log.Println("saved assets to", path)
		}(entry)
	}

	wg.Wait()
}

func downloadAssetsFromSteam(cmdPath string) error {
	var args []string
	args = append(args, "-app")
	args = append(args, os.Getenv("DOWNLOADER_APP_ID"))
	args = append(args, "-depot")
	args = append(args, os.Getenv("DOWNLOADER_DEPOT_ID"))
	args = append(args, "-username")
	args = append(args, os.Getenv("DOWNLOADER_STEAM_USERNAME"))
	args = append(args, "-password")
	args = append(args, os.Getenv("DOWNLOADER_STEAM_PASSWORD"))
	args = append(args, "-filelist")
	args = append(args, "filelist.txt")
	args = append(args, "-dir")
	args = append(args, "./downloaded")

	cmd := exec.Command(cmdPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return err
	}

	if cmd.ProcessState.ExitCode() != 0 {
		return fmt.Errorf("bad exit code from downloader: %d", cmd.ProcessState.ExitCode())
	}
	return nil
}

func (d *parsedData) Add(category, key, value string) {
	if d.data == nil {
		d.data = make(map[string]map[string]string)
	}
	if d.data[category] == nil {
		d.data[category] = make(map[string]string)
	}

	d.data[category][key] = value
}

func (d *parsedData) Encode(w io.Writer) error {
	return yaml.NewEncoder(w).Encode(d.data)
}

func parseAndSaveFile(encrypted []byte, outPath string) error {
	raw, err := decryptDVPL(encrypted)
	if err != nil {
		return err
	}

	data, err := decodeYAML(bytes.NewBuffer(raw))
	if err != nil {
		return err
	}

	var parsed parsedData

	for key, value := range data {
		if key.IsMap() {
			parsed.Add("maps", key.MapName(), value)
		}
		if key.IsBattleType() {
			parsed.Add("battleTypes", key.BattleTypeName(), value)
		}
	}

	err = os.MkdirAll(filepath.Dir(outPath), os.ModePerm)
	if err != nil {
		return err
	}

	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	err = parsed.Encode(f)
	if err != nil {
		return err
	}
	return nil
}

func (k parsedKey) IsMap() bool {
	return strings.HasPrefix(string(k), "#maps:") && strings.HasSuffix(string(k), ".sc2")
}

func (k parsedKey) MapName() string {
	split := strings.Split(string(k), ":")
	if len(split) < 2 {
		return ""
	}
	return split[1]
}

func (k parsedKey) IsBattleType() bool {
	return strings.HasPrefix(string(k), "battleType/") && len(strings.Split(string(k), "/")) == 2
}

func (k parsedKey) BattleTypeName() string {
	return strings.Split(string(k), "/")[1]
}

func decryptDVPL(inputBuf []byte) ([]byte, error) {
	dataBuf := inputBuf[:len(inputBuf)-20]
	footerBuf := inputBuf[len(inputBuf)-20:]

	originalSize := binary.LittleEndian.Uint32(footerBuf[:4])
	compressedSize := binary.LittleEndian.Uint32(footerBuf[4:8])
	if int(compressedSize) != len(dataBuf) {
		return nil, errors.New("invalid compressed data length")
	}

	crc32DataSum := binary.LittleEndian.Uint32(footerBuf[8:12])
	if crc32DataSum != crc32.ChecksumIEEE(dataBuf) {
		return nil, errors.New("invalid crc32 sum")
	}

	compressType := binary.LittleEndian.Uint32(footerBuf[12:16])
	outputBuf := make([]byte, originalSize)
	if compressType == 0 {
		outputBuf = dataBuf
	} else {
		actualOutputSize, err := lz4.UncompressBlock(dataBuf, outputBuf)
		if err != nil {
			return nil, errors.New("failed to uncompressed lz4")
		}
		outputBuf = outputBuf[:actualOutputSize]
	}
	return outputBuf, nil
}

func decodeYAML(r io.Reader) (map[parsedKey]string, error) {
	decoded := make(map[parsedKey]string)
	return decoded, yaml.NewDecoder(r).Decode(&decoded)
}
