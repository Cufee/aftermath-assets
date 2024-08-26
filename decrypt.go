package main

import (
	"encoding/binary"
	"errors"
	"hash/crc32"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/pierrec/lz4/v4"
)

var decryptExtensions = []string{".yaml", ".xml", ".txt"}

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

func decryptDir(path string, dir []fs.DirEntry, outDir string) error {
	var wg sync.WaitGroup
	for _, entry := range dir {
		wg.Add(1)
		go func() {
			defer wg.Done()

			entryPath := filepath.Join(path, entry.Name())

			if !entry.IsDir() {
				err := decryptFile(entryPath, outDir)
				if err != nil {
					log.Println("failed to decrypt a file", entryPath, err)
				}
				return
			}

			dir, err := os.ReadDir(entryPath)
			if err != nil {
				log.Println("failed to read a directory", entryPath, err)
				return
			}

			err = decryptDir(entryPath, dir, filepath.Join(outDir, entry.Name()))
			if err != nil {
				log.Println("failed to decrypt a directory", entryPath, err)
			}
		}()
	}

	wg.Wait()
	return nil
}

func decryptFile(path string, outDir string) error {
	cleanPath, ok := strings.CutSuffix(path, ".dvpl")
	if !ok {
		// log.Println("skipping - not encrypted:", path)
		return nil
	}

	ext := filepath.Ext(cleanPath)
	if !slices.Contains(decryptExtensions, ext) {
		// log.Println("skipping - extension not registered:", path)
		return nil
	}
	log.Println("decrypting", path)

	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	err = os.MkdirAll(outDir, os.ModePerm)
	if err != nil {
		return err
	}

	decrypted, err := decryptDVPL(raw)
	if err != nil {
		return err
	}

	err = os.WriteFile(filepath.Join(outDir, filepath.Base(cleanPath)), decrypted, os.ModePerm)
	if err != nil {
		return err
	}
	return nil
}
