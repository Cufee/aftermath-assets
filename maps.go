package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"golang.org/x/text/language"
)

type Map struct {
	ID              string                  `json:"id"`
	Key             string                  `json:"key"`
	GameModes       []int                   `yaml:"availableModes"`
	SupremacyPoints int                     `yaml:"supremacyPointsThreshold"`
	LocalizedNames  map[language.Tag]string `json:"names"`
}

type mapsEntry struct {
	LocalID         int    `yaml:"id"`
	Key             string `yaml:"localName"`
	GameModes       []int  `yaml:"availableModes"`
	SupremacyPoints int    `yaml:"supremacyPointsThreshold"`
}

type mapParser struct {
	globalLock     *sync.Mutex
	maps           map[string]mapsEntry
	localizedNames map[string]map[language.Tag]string
}

func (p *mapParser) Export(filePath string) error {
	err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm)
	if err != nil {
		return errors.Wrap(err, "failed to create path")
	}

	maps := make(map[string]Map)
	var keys []string
	for key, data := range p.maps {
		m := Map{
			ID:              fmt.Sprint(data.LocalID),
			Key:             data.Key,
			GameModes:       data.GameModes,
			SupremacyPoints: data.SupremacyPoints,
			LocalizedNames:  p.localizedNames[key],
		}
		maps[m.ID] = m
		keys = append(keys, m.ID)
	}

	mapsSorted := make(map[string]Map)
	sort.Strings(keys)
	for _, key := range keys {
		mapsSorted[key] = maps[key]
	}

	f, err := os.Create(filePath)
	if err != nil {
		return errors.Wrap(err, "failed to create file")
	}

	e := json.NewEncoder(f)
	e.SetIndent("", "  ")
	return e.Encode(mapsSorted)
}

func (p *mapParser) Strings() *mapStringsParser {
	return &mapStringsParser{p.globalLock, p.maps, p.localizedNames}
}

func (p *mapParser) Maps() *mapDictParser {
	return &mapDictParser{p.maps, p.globalLock}
}

func newMapParser() *mapParser {
	return &mapParser{
		globalLock:     &sync.Mutex{},
		maps:           map[string]mapsEntry{},
		localizedNames: map[string]map[language.Tag]string{},
	}
}

type mapStringsParser struct {
	lock           *sync.Mutex
	maps           map[string]mapsEntry
	localizedNames map[string]map[language.Tag]string
}

func (p *mapStringsParser) Exclusive() bool {
	return false
}
func (p *mapStringsParser) Match(path string) bool {
	return stringsRegex.MatchString(path)
}
func (p *mapStringsParser) Parse(path string, r io.Reader) error {
	lang := strings.Split(filepath.Base(path), ".")[0]
	locale, err := language.Parse(lang)
	if err != nil {
		return errors.Wrap(err, "failed to get locale from a filename")
	}

	data, err := decodeYAML[map[string]string](r)
	if err != nil {
		return err
	}

	p.lock.Lock()
	defer p.lock.Unlock()

	for key, value := range data {
		if !strings.HasPrefix(key, "#maps:") {
			continue
		}

		for name, data := range p.maps {
			if fmt.Sprintf("#maps:%s:%s", name, data.Key) != key {
				continue
			}
			mapNames, ok := p.localizedNames[name]
			if !ok {
				mapNames = make(map[language.Tag]string)
			}
			mapNames[locale] = value
			p.localizedNames[name] = mapNames
		}
	}

	return nil
}

type mapDictParser struct {
	maps     map[string]mapsEntry
	mapsLock *sync.Mutex
}

func (p *mapDictParser) Exclusive() bool {
	return true
}
func (p *mapDictParser) Match(path string) bool {
	return strings.HasSuffix(path, "maps.yaml")
}
func (p *mapDictParser) Parse(path string, r io.Reader) error {
	data, err := decodeYAML[struct {
		Maps map[string]mapsEntry `yaml:"maps"`
	}](r)
	if err != nil {
		return err
	}

	p.mapsLock.Lock()
	for key, value := range data.Maps {
		p.maps[key] = value
	}
	p.mapsLock.Unlock()

	return nil
}
