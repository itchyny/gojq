package cli

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/itchyny/gojq"
)

type moduleLoader struct {
	paths []string
}

func (l *moduleLoader) LoadInitModules() ([]*gojq.Module, error) {
	var ms []*gojq.Module
	for _, path := range l.paths {
		if filepath.Base(path) != ".jq" {
			continue
		}
		fi, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		if fi.IsDir() {
			continue
		}
		cnt, err := ioutil.ReadFile(path)
		if err != nil {
			return nil, err
		}
		m, err := gojq.ParseModule(string(cnt))
		if err != nil {
			return nil, &queryParseError{"query in module", path, string(cnt), err}
		}
		ms = append(ms, m)
	}
	return ms, nil
}

func (l *moduleLoader) LoadModule(name string) (*gojq.Module, error) {
	path, err := l.lookupModule(name, ".jq")
	if err != nil {
		return nil, err
	}
	cnt, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	m, err := gojq.ParseModule(string(cnt))
	if err != nil {
		return nil, &queryParseError{"query in module", path, string(cnt), err}
	}
	return m, nil
}

func (l *moduleLoader) LoadJSON(name string) (interface{}, error) {
	path, err := l.lookupModule(name, ".json")
	if err != nil {
		return nil, err
	}
	vals, err := slurpFile(path)
	if err != nil {
		return nil, err
	}
	return vals, nil
}

func (l *moduleLoader) lookupModule(name, extension string) (string, error) {
	for _, base := range l.paths {
		path := filepath.Clean(filepath.Join(base, name+extension))
		if _, err := os.Stat(path); err == nil {
			return path, err
		}
		path = filepath.Clean(filepath.Join(base, name, filepath.Base(name)+extension))
		if _, err := os.Stat(path); err == nil {
			return path, err
		}
	}
	return "", fmt.Errorf("module not found: %q", name)
}
