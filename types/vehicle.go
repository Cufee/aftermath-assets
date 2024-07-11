package types

import "golang.org/x/text/language"


type Vehicle struct {
	ID             string                  `json:"id"`
	Key            string                  `json:"key"`
	LocalizedNames map[language.Tag]string `json:"names"`

	Tier        int    `json:"tier"`
	Class       string `json:"class"`
	Nation      string `json:"nation"`
	Premium     bool   `json:"premium"`
	SuperTest   bool   `json:"superTest"`
	Collectible bool   `json:"collectible"`
}
