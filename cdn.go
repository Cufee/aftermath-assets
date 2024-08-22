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
			for _, v := range response.Data {
				vehicle, ok := glossary[fmt.Sprint(v.ID)]
				if !ok {
					vehicle = make(map[language.Tag]vehicleRecord)
				}
				t, _ := language.Parse(locale)
				vehicle[t] = v
				glossary[fmt.Sprint(v.ID)] = vehicle
			}
			glossaryLock.Unlock()
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
