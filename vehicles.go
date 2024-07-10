package main

import (
	"encoding/json"

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
	return &vehicleStringsParser{p.vehicles, out}
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
	Price        string `xml:"price" json:"price"`

	id           int
	level        int
	tags         []string
	environments []string
}

var vehicleClasses = []string{"AT-SPG"}

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
		Premium:     strings.Contains(item.Price, "<gold/>"),
		SuperTest:   slices.Contains(item.environments, "supertest"),
		Collectible: slices.Contains(item.tags, "collectible"),
	}
}

func (p *vehicleItemsParser) Exclusive() bool {
	return true
}
func (p *vehicleItemsParser) Match(path string) bool {
	return vehicleItemsRegex.MatchString(path)
}
func (p *vehicleItemsParser) Parse(path string, r io.Reader) error {
	data, err := decodeXML[map[string]vehicleItem](r)
	if err != nil {
		return err
	}

	nation := filepath.Base(filepath.Dir(path))
	if slices.Contains([]string{"provisions", "consumables"}, nation) {
		return nil
	}
	if _, ok := nationIDs[nation]; !ok {
		return errors.New("invalid nation " + nation)
	}

	p.lock.Lock()
	defer p.lock.Unlock()

	for _, item := range data {
		if item.ID == "" {
			continue
		}
		vehicle := item.toVehicle(nation)
		p.vehicles[vehicle.ID] = vehicle
	}

	return nil
}

type vehicleStringsParser struct {
	vehicles  map[string]Vehicle
	assetsDir string
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

	// for _, vehicle := range data {

	// }

	_ = data

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

	return nil
}
