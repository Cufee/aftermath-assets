package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"
)

type Map struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

type mapsEntry struct {
	LocalID         int    `yaml:"id"`
	Key             string `yaml:"localName"`
	GameModes       []int  `yaml:"availableModes"`
	SupremacyPoints int    `yaml:"supremacyPointsThreshold"`
}

type mapParser struct {
	maps     map[string]mapsEntry
	mapsLock *sync.Mutex
}

func (p *mapParser) Strings(out string) *mapStringsParser {
	return &mapStringsParser{p.maps, out}
}

func (p *mapParser) Maps() *mapDictParser {
	return &mapDictParser{p.maps, &sync.Mutex{}}
}

func newMapParser() *mapParser {
	return &mapParser{
		mapsLock: &sync.Mutex{},
		maps:     map[string]mapsEntry{},
	}
}

type mapStringsParser struct {
	maps      map[string]mapsEntry
	assetsDir string
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

	err = os.MkdirAll(filepath.Join(p.assetsDir, locale.String()), os.ModePerm)
	if err != nil {
		return errors.Wrap(err, "failed to create a subpath in assets directory")
	}

	data, err := decodeYAML[map[string]string](r)
	if err != nil {
		return err
	}

	var mapsData []Map

	for key, value := range data {
		if !strings.HasPrefix(key, "#maps:") {
			continue
		}

		for name, data := range p.maps {
			if fmt.Sprintf("#maps:%s:%s", name, data.Key) != key {
				continue
			}
			mapsData = append(mapsData, Map{
				ID:   fmt.Sprint(data.LocalID),
				Key:  name,
				Name: value,
			})
		}
	}

	// Save data as JSON
	jf, err := os.Create(filepath.Join(p.assetsDir, locale.String(), "maps.json"))
	if err != nil {
		return err
	}
	defer jf.Close()

	je := json.NewEncoder(jf)
	je.SetIndent("", "  ")
	err = je.Encode(mapsData)
	if err != nil {
		return err
	}

	// Save data as YAML
	var yamlData []LocalizationString
	for _, data := range mapsData {
		yamlData = append(yamlData, LocalizationString{
			Key:   "maps_" + data.ID,
			Value: data.Name,
		})
		yamlData = append(yamlData, LocalizationString{
			Key:   "maps_" + data.Key,
			Value: data.Name,
		})
	}

	yf, err := os.Create(filepath.Join(p.assetsDir, locale.String(), "maps.yaml"))
	if err != nil {
		return err
	}
	defer yf.Close()

	ye := yaml.NewEncoder(yf)
	err = ye.Encode(yamlData)
	if err != nil {
		return nil
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
