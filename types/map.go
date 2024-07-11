package types

import "golang.org/x/text/language"

type Map struct {
	ID              string                  `json:"id"`
	Key             string                  `json:"key"`
	GameModes       []int                   `yaml:"availableModes"`
	SupremacyPoints int                     `yaml:"supremacyPointsThreshold"`
	LocalizedNames  map[language.Tag]string `json:"names"`
}
