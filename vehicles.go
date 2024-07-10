package main

import (
	"bytes"
	"encoding/json"
	"strconv"

	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"
)

type Vehicle struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`

	Tier        int    `json:"tier"`
	Class       string `json:"class"`
	Nation      string `json:"nation"`
	Premium     bool   `json:"premium"`
	SuperTest   bool   `json:"superTest"`
	Collectible bool   `json:"collectible"`
}

type vehiclesParser struct {
	vehicles map[string]Vehicle
	lock     *sync.Mutex
}

func newVehiclesParser() *vehiclesParser {
	return &vehiclesParser{
		lock:     &sync.Mutex{},
		vehicles: make(map[string]Vehicle),
	}
}

func (p *vehiclesParser) Items() *vehicleItemsParser {
	return &vehicleItemsParser{p.vehicles, p.lock}
}
func (p *vehiclesParser) Strings(out string) *vehicleStringsParser {
	return &vehicleStringsParser{out, p.vehicles, p.lock}
}

var vehicleItemsRegex = regexp.MustCompile(".*/XML/item_defs/vehicles/.*list.xml")

type vehicleItemsParser struct {
	vehicles map[string]Vehicle
	lock     *sync.Mutex
}

type vehicleItem struct {
	ID           string `xml:"id" json:"id"`
	Name         string `xml:"userString" json:"userString"`
	NameShort    string `xml:"shortUserString" json:"shortUserString"`
	Tags         string `xml:"tags" json:"tags"`
	Level        string `xml:"level" json:"level"`
	Environments string `xml:"configurationModes" json:"configurationModes"`

	id           int
	level        int
	tags         []string
	environments []string
}

var vehicleClasses = []string{"AT-SPG", "lightTank", "mediumTank", "heavyTank"}

func (item vehicleItem) class() string {
	for _, tag := range item.tags {
		if slices.Contains(vehicleClasses, tag) {
			return tag
		}
	}
	return "unknown"
}

func (item vehicleItem) toVehicle(nation string) Vehicle {
	key := item.Name
	if item.NameShort != "" {
		key = item.NameShort
	}

	return Vehicle{
		Key:  key,
		Name: fmt.Sprintf("Secret Tank %d", item.id), // this will be updated from strings later
		ID:   fmt.Sprint(toGlobalID(nation, item.id)),

		Tier:        item.level,
		Class:       item.class(),
		Nation:      nation,
		SuperTest:   slices.Contains(item.environments, "supertest") && !slices.Contains(item.environments, "production"),
		Collectible: slices.Contains(item.tags, "collectible"),
	}
}

func (p *vehicleItemsParser) Exclusive() bool {
	return true
}
func (p *vehicleItemsParser) Match(path string) bool {
	return vehicleItemsRegex.MatchString(path) && !strings.HasSuffix(path, "provisions/list.xml") && !strings.HasSuffix(path, "consumables/list.xml")
}
func (p *vehicleItemsParser) Parse(path string, r io.Reader) error {
	raw, _ := io.ReadAll(r)
	data, err := decodeXML[map[string]vehicleItem](bytes.NewReader(raw))
	if err != nil {
		return err
	}

	nation := filepath.Base(filepath.Dir(path))
	if _, ok := nationIDs[nation]; !ok {
		return errors.New("invalid nation " + nation)
	}

	p.lock.Lock()
	defer p.lock.Unlock()

	for _, item := range data {
		item.id, _ = strconv.Atoi(item.ID)
		item.level, _ = strconv.Atoi(item.Level)
		item.tags = strings.Split(item.Tags, " ")
		item.environments = strings.Split(item.Environments, " ")

		vehicle := item.toVehicle(nation)
		p.vehicles[vehicle.ID] = vehicle
	}

	return nil
}

type vehicleStringsParser struct {
	assetsDir string
	vehicles  map[string]Vehicle
	lock      *sync.Mutex
}

func (p *vehicleStringsParser) Exclusive() bool {
	return true
}
func (p *vehicleStringsParser) Match(path string) bool {
	return stringsRegex.MatchString(path)
}

func (p *vehicleStringsParser) Parse(path string, r io.Reader) error {
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

	p.lock.Lock()
	defer p.lock.Unlock()
	var vehicles []LocalizationString
	for key, vehicle := range p.vehicles {
		v := p.vehicles[key]
		v.Name = data[vehicle.Key]
		p.vehicles[key] = v

		vehicles = append(vehicles, LocalizationString{
			Key:   "vehicle_" + v.ID,
			Value: v.Name,
			Notes: v.Key,
		})
	}

	// Save data as JSON
	jf, err := os.Create(filepath.Join(p.assetsDir, locale.String(), "vehicles.json"))
	if err != nil {
		return err
	}
	defer jf.Close()

	je := json.NewEncoder(jf)
	je.SetIndent("", "  ")
	err = je.Encode(p.vehicles)
	if err != nil {
		return err
	}

	// Save data as yaml
	yf, err := os.Create(filepath.Join(p.assetsDir, locale.String(), "vehicles.yaml"))
	if err != nil {
		return err
	}
	defer yf.Close()

	ye := yaml.NewEncoder(yf)
	err = ye.Encode(vehicles)
	if err != nil {
		return nil
	}

	return nil
}
