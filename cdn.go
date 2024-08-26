package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"golang.org/x/text/language"
)

var cdnLanguages = []string{"en", "ru", "pl", "de", "fr", "es", "zh-cn", "zh-tw", "tr", "cs", "th", "vi", "ko"}

type wargamingCDNClient struct {
	applicationID string
}

func NewCDNClient(applicationID string) *wargamingCDNClient {
	return &wargamingCDNClient{applicationID: applicationID}
}

type vehicleRecord struct {
	Premium bool   `json:"is_premium"`
	Tier    int    `json:"tier"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	ID      int    `json:"tank_id"`
}

func (c *wargamingCDNClient) Vehicles(locales ...string) (map[string]map[language.Tag]vehicleRecord, error) {
	if c.applicationID == "" {
		return nil, errors.New("missing application id")
	}

	if len(locales) == 0 {
		locales = append(locales, "en")
	}

	var glossary = make(map[string]map[language.Tag]vehicleRecord)

	var wg sync.WaitGroup
	var glossaryLock sync.Mutex
	errorCh := make(chan error)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()

	for _, l := range locales {
		wg.Add(1)
		go func(locale string) {
			defer wg.Done()

			req, err := http.NewRequest("GET", "https://api.wotblitz.eu/wotb/encyclopedia/vehicles/?fields=name%2Cis_premium%2Ctier%2Ctank_id%2Ctype&application_id="+c.applicationID, nil)
			if err != nil {
				errorCh <- err
				return
			}

			res, err := http.DefaultClient.Do(req.WithContext(ctx))
			if err != nil {
				errorCh <- err
				return
			}
			defer res.Body.Close()

			var response struct {
				Data map[string]vehicleRecord `json:"data"`
			}
			err = json.NewDecoder(res.Body).Decode(&response)
			if err != nil {
				errorCh <- err
				return
			}

			glossaryLock.Lock()
			defer glossaryLock.Unlock()
			for _, v := range response.Data {
				vehicle, ok := glossary[fmt.Sprint(v.ID)]
				if !ok {
					vehicle = make(map[language.Tag]vehicleRecord)
				}
				t, err := language.Parse(locale)
				if err != nil {
					errorCh <- err
					return
				}
				vehicle[t] = v
				glossary[fmt.Sprint(v.ID)] = vehicle
			}
		}(l)
	}

	wgDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(wgDone)
	}()

	select {
	case <-wgDone:
		close(errorCh)
		break
	case err := <-errorCh:
		return nil, err
	}

	return glossary, nil
}

func (c *wargamingCDNClient) MissingStrings(locales ...string) (map[language.Tag]map[string]string, error) {
	if len(locales) == 0 {
		locales = append(locales, "en")
	}

	var strings = make(map[language.Tag]map[string]string)

	var lock sync.Mutex
	var wg sync.WaitGroup
	errorCh := make(chan error)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()

	for _, l := range locales {
		wg.Add(1)
		go func(locale string) {
			defer wg.Done()
			req, err := http.NewRequest("GET", fmt.Sprintf("https://stufficons.wgcdn.co/localizations/%v.yaml", locale), nil)
			if err != nil {
				errorCh <- err
				return
			}

			res, err := http.DefaultClient.Do(req.WithContext(ctx))
			if err != nil {
				errorCh <- err
				return
			}
			defer res.Body.Close()

			if res.StatusCode != 200 {
				println("failed to get missing localization strings for "+locale, res.Status)
				return
			}

			data, err := decodeYAML[map[string]string](res.Body)
			if err != nil {
				errorCh <- err
				return
			}

			lock.Lock()
			defer lock.Unlock()
			t, err := language.Parse(locale)
			if err != nil {
				errorCh <- err
				return
			}

			strings[t] = data
		}(l)
	}

	wgDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(wgDone)
	}()

	select {
	case <-wgDone:
		close(errorCh)
		break
	case err := <-errorCh:
		return nil, err
	}

	return strings, nil
}
