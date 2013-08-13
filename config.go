package main

import (
	"bytes"
	"errors"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type MopidyConfig struct {
	Data map[string] string
	Path string
}

// LoadConfig loads the application's mopidy configuration and, if none is found,
// creates a new one using `mopidy --show-config`. The configuration is represented
// as a key-value map, with keys being of the form "section/key" e.g. "spotify/username".
func LoadConfig(mopidy, userConfigPath string) (*MopidyConfig, error) {
	cfg := &MopidyConfig{Path:userConfigPath}
	cfg.Data = make(map[string] string)
	var body []byte

	if _, err := os.Stat(userConfigPath); os.IsNotExist(err) {
		// the config file isn't there, so create one based on mopidy's defaults
		if body, err = exec.Command(mopidy, "--show-config").Output(); err != nil {
			return nil, errors.New("'mopidy --show-config' failed: " + err.Error())
		}

		if err = os.MkdirAll(filepath.Dir(userConfigPath), 0777); err != nil {
			return nil, errors.New("failed to create config directory: " + err.Error())
		}

		if err = ioutil.WriteFile(userConfigPath, body, 0777); err != nil {
			return nil, errors.New("failed to create config file: " + err.Error())
		}
	} else {
		if data, err := ioutil.ReadFile(userConfigPath); err == nil {
			body = data
		} else {
			return nil, errors.New("failed to read config file: " + err.Error())
		}
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
			section = line[1:len(line)-1]
			continue
		}

		if i := strings.Index(line, "="); i != -1 {
			// we have a key, so save it
			key = section + "/" + strings.TrimSpace(line[:i])
			value := strings.TrimSpace(line[i+1:])
			cfg.Data[key] = value
		} else {
			// append it to the previous key
			if cfg.Data[key] == "" {
				cfg.Data[key] = line
			} else {
				cfg.Data[key] = cfg.Data[key] + "," + line
			}
		}
	}
	return cfg, nil
}

// Get gets the value of a configuration key. It's analogous to Go's native map access.
func (cfg *MopidyConfig) Get(key string) (string, bool) {
	val, found := cfg.Data[key]
	return val, found
}

// GetBool gets the value of a configuration key and converts it to a boolean. If the
// key is found but fails to convert to a boolean, then this method returns an error.
func (cfg *MopidyConfig) GetBool(key string) (bool, bool, error) {
	val, found := cfg.Data[key]
	if !found {
		return false, found, nil
	}
	b, err := strconv.ParseBool(val)
	return b, found, err
}

// Set sets a value in the configuration.
func (cfg *MopidyConfig) Set(key, value string) {
	cfg.Data[key] = value
}

// SetBool is the same as Set, but it takes a boolean instead.
func (cfg *MopidyConfig) SetBool(key string, value bool) {
	cfg.Data[key] = strconv.FormatBool(value)
}

// Save saves the configuration to the application's mopidy config file.
func (cfg *MopidyConfig) Save() error {
	sections := make(map[string] []string)

	for key, value := range cfg.Data {
		if i := strings.Index(key, "/"); i != -1 {
			section := key[:i]
			line := key[i+1:] + "=" + value
			if _, ok := sections[section]; !ok {
				sections[section] = make([]string, 0)
			}
			sections[section] = append(sections[section], line)
		} else {
			return errors.New("invalid configuration key found: " + key)
		}
	}

	var buffer bytes.Buffer
	for key, value := range sections {
		buffer.WriteString("[" + key + "]\n")
		for _, line := range value {
			buffer.WriteString(line + "\n")
		}
		buffer.WriteString("\n")
	}

	if err := ioutil.WriteFile(cfg.Path, buffer.Bytes(), 0777); err != nil {
		return errors.New("failed to write to config file: " + err.Error())
	}

	return nil
}

