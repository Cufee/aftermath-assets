package main

import (
	"bytes"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/pkg/errors"
)

type parseFunc interface {
	Parse(path string, r io.Reader) error

	Exclusive() bool
	Match(path string) bool
}

type parser struct {
	inputPath string
	parsers   []parseFunc
}

func newParser(in string, parsers ...parseFunc) (*parser, error) {
	if len(parsers) < 1 {
		return nil, errors.New("parsers slice cannot be empty")
	}
	return &parser{inputPath: in, parsers: parsers}, nil
}

func (p *parser) Parse() error {
	return p.parseDir(p.inputPath)
}

func (p *parser) parseDir(path string) error {
	dir, err := os.ReadDir(path)
	if err != nil {
		return errors.Wrap(err, "failed to read a directory")
	}

	var wg sync.WaitGroup
	for _, entry := range dir {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fullPath := filepath.Join(path, entry.Name())

			if !entry.IsDir() {
				err := p.parseFile(fullPath)
				if err != nil {
					log.Println(fullPath, err)
				}
				return
			}

			err = p.parseDir(fullPath)
			if err != nil {
				log.Println(fullPath, err)
			}
		}()
	}
	wg.Wait()

	return nil
}

func (p *parser) parseFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	for _, parser := range p.parsers {
		if parser.Match(path) {
			log.Println("parsing", path)

			err = parser.Parse(path, bytes.NewBuffer(data))
			if parser.Exclusive() {
				return errors.Wrap(err, "failed to parse a file")
			}
			if err != nil {
				log.Println(path, errors.Wrap(err, "failed to parse a file"))
			}
		}
	}

	return nil
}
