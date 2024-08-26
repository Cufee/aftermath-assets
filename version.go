package main

import (
	"encoding/json"

	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

type versionParser struct {
	Tag         string `json:"tag"`
	Arch        string `json:"arch"`
	GameVersion string `json:"gameVersion"`
}

func newVersionParser() *versionParser {
	return &versionParser{}
}

func (p *versionParser) Parse(path string, r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	// 11.1.0.743_4002766 release/11.1.0 WOTB_Win7
	version := strings.Split(string(data), " ")
	if len(version) != 3 {
		return errors.New("invalid version.txt, split length should be 3")
	}
	p.Tag = version[0]
	p.GameVersion = strings.TrimPrefix(version[1], "release/")
	p.Arch = strings.ToLower(strings.TrimPrefix(version[2], "WOTB_"))

	return nil
}

func (p *versionParser) Exclusive() bool {
	return true
}

func (p *versionParser) Match(path string) bool {
	return strings.HasSuffix(path, "version.txt")
}

func (p *versionParser) Export(filePath string) error {
	err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm)
	if err != nil {
		return errors.Wrap(err, "failed to create path")
	}

	f, err := os.Create(filePath)
	if err != nil {
		return errors.Wrap(err, "failed to create file")
	}

	e := json.NewEncoder(f)
	e.SetIndent("", "  ")
	return e.Encode(p)
}
