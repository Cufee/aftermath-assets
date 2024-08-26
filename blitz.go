package main

func toGlobalID(nation string, id int) int {
	nid, ok := nationIDs[nation]
	if !ok {
		nid = lastNation + 1
	}
	return (id << 8) + (nid << 4) + 1
}

const lastNation int = 8

var nationIDs = map[string]int{
	"ussr":     0,
	"germany":  1,
	"usa":      2,
	"china":    3,
	"france":   4,
	"uk":       5,
	"japan":    6,
	"other":    7,
	"european": lastNation,
}
