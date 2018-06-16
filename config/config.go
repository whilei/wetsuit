package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
)

type Properties struct {
	data map[string]interface{}
	path string
}

// Path() returns the path that was used to construct this Properties instance.
func (p *Properties) Path() string {
	return p.path
}

// Get() returns the value associated with the given key, returning an error
// if it's not found or is not a string.
func (p *Properties) Get(key string) (string, error) {
	val, ok := p.data[key]
	if !ok {
		return "", &NotFoundError{key: key}
	}
	str, ok := val.(string)
	if !ok {
		return "", &InvalidTypeError{typ: "string"}
	}
	return str, nil
}

// Set() sets a value in the configuration.
func (p *Properties) Set(key, value string) {
	p.data[key] = value
}

// GetBool() is the same as Get(), but expects a boolean instead.
func (p *Properties) GetBool(key string) (bool, error) {
	val, ok := p.data[key]
	if !ok {
		return false, &NotFoundError{key: key}
	}
	b, ok := val.(bool)
	if !ok {
		return false, &InvalidTypeError{typ: "bool"}
	}
	return b, nil
}

// SetBool() is the same as Set(), but it takes a boolean instead.
func (p *Properties) SetBool(key string, value bool) {
	p.data[key] = value
}

// Load() takes a config file path and loads it into a new Properties struct.
func Load(path string) (*Properties, error) {
	p := new(Properties)
	p.data = make(map[string]interface{})
	p.path = path

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("file not found: '%s'", path)
	}

	var body []byte
	if data, err := ioutil.ReadFile(path); err == nil {
		body = data
	} else {
		return nil, fmt.Errorf("could not read '%s'", path)
	}

	// now we have the body, so parse it into the map
	lines := strings.Split(string(body), "\n")
	var section, key string

	for _, line := range lines {
		// look for comments
		if i := strings.Index(line, "#"); i != -1 {
			line = line[0:i]
		}

		line = strings.TrimSpace(line)

		// skip empty lines
		if len(line) == 0 {
			continue
		}

		// check for the start of a section
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = line[1 : len(line)-1]
			continue
		}

		if i := strings.Index(line, "="); i != -1 {
			// we have a key, so save it
			key = section + "/" + strings.TrimSpace(line[:i])
			value := strings.TrimSpace(line[i+1:])
			if b, err := strconv.ParseBool(value); err == nil {
				p.data[key] = b
			} else {
				p.data[key] = value
			}
		} else {
			// append it to the previous key
			if p.data[key] == "" {
				p.data[key] = line
			} else {
				p.data[key] = p.data[key].(string) + "," + line
			}
		}
	}

	return p, nil
}

// NotFoundError is returned from a Get method if the key wasn't found.
type NotFoundError struct {
	key string
}

// Error() returns the error's message.
func (e *NotFoundError) Error() string {
	return fmt.Sprintf("key '%s' not found", e.key)
}

// Key() returns the name of the key that wasn't found.
func (e *NotFoundError) Key() string {
	return e.key
}

// InvalidTypeError is returned from a Get method if the key was found,
// but it was of an unexpected type.
type InvalidTypeError struct {
	typ string
}

// Error() returns the error's message.
func (e *InvalidTypeError) Error() string {
	return fmt.Sprintf("key found, but it was the wrong type; expected '%s'", e.typ)
}
