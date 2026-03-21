package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
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
	glossary     map[string]map[language.Tag]vehicleRecord
}

func newVehiclesParser(glossary map[string]map[language.Tag]vehicleRecord) *vehiclesParser {
	return &vehiclesParser{
		glossary:     glossary,
		lock:         &sync.Mutex{},
		vehicles:     make(map[string]types.Vehicle),
		vehicleNames: make(map[string]map[language.Tag]string),
	}
}

func (p *vehiclesParser) Items() *vehicleItemsParser {
	return &vehicleItemsParser{p.glossary, p.vehicles, p.lock}
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
	err = e.Encode(vehiclesSorted)
	if err != nil {
		return err
	}

	return nil
}

// parseGoldPriceVehicles parses raw XML to find vehicles whose <price> contains
// a <gold/> sub-element. These are premium/collectible vehicles.
func parseGoldPriceVehicles(raw []byte) map[string]bool {
	result := make(map[string]bool)
	decoder := xml.NewDecoder(bytes.NewReader(raw))

	var currentVehicle string
	var inPrice bool
	var depth int // depth inside <root>

	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "root" {
				continue
			}
			if depth == 0 {
				// direct child of <root> = vehicle name
				currentVehicle = t.Name.Local
				depth = 1
			} else if depth == 1 && t.Name.Local == "price" {
				inPrice = true
			} else if inPrice && t.Name.Local == "gold" {
				result[currentVehicle] = true
			}
		case xml.EndElement:
			if t.Name.Local == "root" {
				continue
			}
			if t.Name.Local == currentVehicle {
				currentVehicle = ""
				depth = 0
				inPrice = false
			} else if t.Name.Local == "price" {
				inPrice = false
			}
		}
	}

	return result
}

var vehicleItemsRegex = regexp.MustCompile(".*/XML/item_defs/vehicles/.*list.xml")

type vehicleItemsParser struct {
	glossary map[string]map[language.Tag]vehicleRecord
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
	premium      bool
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

func (item vehicleItem) toVehicle(id, nation string, glossary map[language.Tag]vehicleRecord) types.Vehicle {
	key := item.Name
	if item.NameShort != "" {
		key = item.NameShort
	}

	names := make(map[language.Tag]string)
	for t, v := range glossary {
		names[t] = v.Name
	}

	return types.Vehicle{
		Key: key,
		ID:  id,

		Tier:        item.level,
		Class:       item.class(),
		Nation:      nation,
		Premium:     item.premium,
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

	// Parse raw XML to determine which vehicles have gold prices (premium indicator).
	// The mxj library strips the <gold/> sub-element from <price>, so we use
	// encoding/xml to detect it from the raw bytes.
	premiumVehicles := parseGoldPriceVehicles(raw)

	p.lock.Lock()
	defer p.lock.Unlock()

	for name, item := range data {
		item.id, _ = strconv.Atoi(item.ID)
		item.level, _ = strconv.Atoi(item.Level)
		item.tags = strings.Split(item.Tags, " ")
		item.environments = strings.Split(item.Environments, " ")
		item.premium = premiumVehicles[name]

		id := fmt.Sprint(toGlobalID(nation, item.id))
		vehicle := item.toVehicle(id, nation, p.glossary[id])
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

var jsonStringsRegex = regexp.MustCompile(".*/Strings/.*.json")

func (p *vehicleStringsParser) Match(path string) bool {
	return jsonStringsRegex.MatchString(path)
}

func (p *vehicleStringsParser) Parse(path string, r io.Reader) error {
	lang := strings.Split(filepath.Base(path), ".")[0]
	locale, err := language.Parse(lang)
	if err != nil {
		return errors.Wrap(err, "failed to get locale from a filename")
	}

	data, err := decodeJSON[map[string]string](r)
	if err != nil {
		return err
	}

	p.lock.Lock()
	defer p.lock.Unlock()

	for key, vehicle := range p.vehicles {
		names := p.vehicleNames[key]
		if names == nil {
			names = make(map[language.Tag]string)
		}
		if localized, ok := data[vehicle.Key]; ok {
			names[locale] = localized
		}
		p.vehicleNames[key] = names
	}

	return nil
}

