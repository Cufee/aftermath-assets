package main

import (
	"bytes"
	"encoding/json"
	"os"
	"sort"
	"strconv"

	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/cufee/aftermath-assets/types"
	"github.com/pkg/errors"
	"golang.org/x/text/language"
)

type vehiclesParser struct {
	vehicleNames map[string]map[language.Tag]string
	vehicles     map[string]types.Vehicle
	lock         *sync.Mutex
}

func newVehiclesParser() *vehiclesParser {
	return &vehiclesParser{
		lock:         &sync.Mutex{},
		vehicles:     make(map[string]types.Vehicle),
		vehicleNames: make(map[string]map[language.Tag]string),
	}
}

func (p *vehiclesParser) Items() *vehicleItemsParser {
	return &vehicleItemsParser{p.vehicles, p.lock}
}
func (p *vehiclesParser) Strings() *vehicleStringsParser {
	return &vehicleStringsParser{p.vehicleNames, p.vehicles, p.lock}
}
func (p *vehiclesParser) Export(filePath string) error {
	err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm)
	if err != nil {
		return errors.Wrap(err, "failed to create path")
	}

	var keys []string
	vehicles := make(map[string]types.Vehicle)
	for key, vehicle := range p.vehicles {
		names := p.vehicleNames[key]

		// reduce the size
		nameEnglish := names[language.English]
		for tag, name := range names {
			if name == nameEnglish && tag != language.English {
				delete(names, tag)
			}
		}

		vehicle.LocalizedNames = names
		vehicles[vehicle.ID] = vehicle
		keys = append(keys, vehicle.ID)
	}

	sort.Strings(keys)
	vehiclesSorted := make(map[string]types.Vehicle)
	for _, key := range keys {
		vehiclesSorted[key] = vehicles[key]
	}

	f, err := os.Create(filePath)
	if err != nil {
		return errors.Wrap(err, "failed to create file")
	}

	e := json.NewEncoder(f)
	e.SetIndent("", "  ")
	return e.Encode(vehiclesSorted)
}

var vehicleItemsRegex = regexp.MustCompile(".*/XML/item_defs/vehicles/.*list.xml")

type vehicleItemsParser struct {
	vehicles map[string]types.Vehicle
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

func (item vehicleItem) toVehicle(nation string) types.Vehicle {
	key := item.Name
	if item.NameShort != "" {
		key = item.NameShort
	}

	return types.Vehicle{
		Key: key,
		// Name: fmt.Sprintf("Secret Tank %d", item.id), // this will be updated from strings later
		ID: fmt.Sprint(toGlobalID(nation, item.id)),

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
	vehicleNames map[string]map[language.Tag]string
	vehicles     map[string]types.Vehicle
	lock         *sync.Mutex
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

	data, err := decodeYAML[map[string]string](r)
	if err != nil {
		return err
	}

	p.lock.Lock()
	defer p.lock.Unlock()

	for key, vehicle := range p.vehicles {
		names, ok := p.vehicleNames[key]
		if !ok {
			names = make(map[language.Tag]string)
		}
		localizedName := data[vehicle.Key]
		if localizedName == "" {
			localizedName = "Secret Tank " + vehicle.ID
		}
		names[locale] = localizedName
		p.vehicleNames[key] = names
	}

	return nil
}
