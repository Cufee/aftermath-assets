package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"golang.org/x/text/language"
)

type battleTypeParser struct {
	typeNamesMx *sync.Mutex
	typeNames   map[string]map[language.Tag]string
}

func newBattleTypeParser() *battleTypeParser {
	return &battleTypeParser{
		typeNamesMx: &sync.Mutex{},
		typeNames:   make(map[string]map[language.Tag]string),
	}
}

func (p *battleTypeParser) Exclusive() bool {
	return false
}

func (p *battleTypeParser) Match(path string) bool {
	return stringsRegex.MatchString(path)
}

func (p *battleTypeParser) Export(filePath string) error {
	err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm)
	if err != nil {
		return errors.Wrap(err, "failed to create path")
	}

	gameModes := make(map[string]map[string]string)
	var keys []string
	for bt, localized := range p.typeNames {
		key := "game_mode_" + bt
		keys = append(keys, key)

		names, ok := gameModes[key]
		if !ok {
			names = make(map[string]string)
		}
		for locale, value := range localized {
			names[locale.String()] = value
		}
		gameModes[key] = names
	}

	gameModesSorted := make(map[string]map[string]string)
	sort.Strings(keys)
	for _, key := range keys {
		gameModesSorted[key] = gameModes[key]
	}

	f, err := os.Create(filePath)
	if err != nil {
		return errors.Wrap(err, "failed to create file")
	}

	e := json.NewEncoder(f)
	e.SetIndent("", "  ")
	return e.Encode(gameModesSorted)
}

func (p *battleTypeParser) Parse(path string, r io.Reader) error {
	lang := strings.Split(filepath.Base(path), ".")[0]
	locale, err := language.Parse(lang)
	if err != nil {
		return errors.Wrap(err, "failed to get locale from a filename")
	}

	data, err := decodeYAML[map[string]string](r)
	if err != nil {
		return err
	}

	p.typeNamesMx.Lock()
	defer p.typeNamesMx.Unlock()

	for key, value := range data {
		if !strings.HasPrefix(key, "battleType/") || len(strings.Split(key, "/")) > 2 {
			continue
		}

		name := strings.ToLower(strings.TrimPrefix(key, "battleType/"))
		localized, ok := p.typeNames[name]
		if !ok {
			localized = make(map[language.Tag]string)
		}
		localized[locale] = value
		p.typeNames[name] = localized
	}

	return nil
}
