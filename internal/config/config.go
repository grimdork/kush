package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// Config is a very small key=value loader from $HOME/.kush_config
type Config map[string]string

// Load reads $HOME/.kush_config and returns a map of keys to values.
func Load() (Config, error) {
	home := os.Getenv("HOME")
	p := filepath.Join(home, ".kush_config")
	f, err := os.Open(p)
	if err != nil {
		return Config{}, err
	}
	defer f.Close()
	c := make(Config)
	s := bufio.NewScanner(f)
	for s.Scan() {
		ln := strings.TrimSpace(s.Text())
		if ln == "" || strings.HasPrefix(ln, "#") {
			continue
		}
		parts := strings.SplitN(ln, "=", 2)
		if len(parts) != 2 {
			continue
		}
		k := strings.TrimSpace(parts[0])
		v := strings.TrimSpace(parts[1])
		c[k] = v
	}
	return c, nil
}

// GetBool returns a boolean interpretation of a key with default false.
func (c Config) GetBool(k string) bool {
	if c == nil {
		return false
	}
	v, ok := c[k]
	if !ok {
		return false
	}
	v = strings.ToLower(strings.TrimSpace(v))
	return v == "1" || v == "true" || v == "yes"
}
