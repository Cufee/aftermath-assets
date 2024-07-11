package main

import (
	"io"
	"regexp"
)

var battleTypeRegex = regexp.MustCompile("Strings/.*.yaml")

type battleTypeParser struct{}

func (p battleTypeParser) Exclusive() bool {
	return false
}
func (p battleTypeParser) Match(path string) bool {
	return battleTypeRegex.MatchString(path)
}
func (p battleTypeParser) Parse(path string, r io.Reader) error {
	// data, err := decodeYAML[map[string]string](r)
	// if err != nil {
	// 	return errors.Wrap(err, "failed to decode file as yaml")
	// }

	return nil
}
