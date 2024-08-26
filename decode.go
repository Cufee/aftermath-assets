package main

import (
	"encoding/json"
	"io"

	"github.com/clbanning/mxj"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

func decodeYAML[T any](r io.Reader) (T, error) {
	var decoded T
	return decoded, yaml.NewDecoder(r).Decode(&decoded)
}

func decodeJSON[T any](r io.Reader) (T, error) {
	var decoded T
	return decoded, json.NewDecoder(r).Decode(&decoded)
}

func decodeXML[T any](r io.Reader) (T, error) {
	var data T

	xmlMap, err := mxj.NewMapXmlReader(r)
	if err != nil {
		return data, errors.Wrap(err, "mxj#NewMapXmlReader")
	}

	var decoded struct {
		Root T `xml:"root"`
	}
	err = xmlMap.Struct(&decoded)
	if err != nil {
		return data, err
	}
	return decoded.Root, nil
}
