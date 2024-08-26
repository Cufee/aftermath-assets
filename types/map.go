package types

import "golang.org/x/text/language"

type Map struct {
	ID              string                  `json:"id"`
	Key             string                  `json:"key"`
	GameModes       []int                   `json:"availableModes"`
	SupremacyPoints int                     `json:"supremacyPointsThreshold"`
	LocalizedNames  map[language.Tag]string `json:"names"`
}
