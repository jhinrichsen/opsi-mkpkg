package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"strings"
)

// toml is ini-like but defines equal sign instead of colon for separating keys and values
// Two .ini packages already exist:
// - github.com/go-ini/ini
// - github.com/vaughan0/go-ini
// Both hardcode the equal sign as key-value separator, so cannot be used as
// OPSI uses colon.
// This implementation expects the optional "[Changelog]" section to be the
// last section in control file.

const (
	separator     = ":"
	sectionPrefix = "["
	sectionSuffix = "]"
)

func isComment(line string) bool {
	return strings.HasPrefix(line, "#") ||
		strings.HasPrefix(line, ";")
}

func isEmpty(line string) bool {
	return len(line) == 0
}

func keyValue(line string) (string, string) {
	parts := strings.Split(line, separator)
	// key-value structure?
	if len(parts) != 2 {
		return "", ""
	}
	key := parts[0]
	value := parts[1]
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	return key, value
}

// keep all entries in a flat structure, keys prepended by section to allow a package_version and a product_version
// All sections and all keys are lowercase
func parse(control io.Reader) (Metadata, error) {
	m := make(Metadata)
	scanner := bufio.NewScanner(control)
	var sctn string
	for scanner.Scan() {
		line := scanner.Text()
		line = trim(line)
		if isComment(line) || isEmpty(line) {
			continue
		}

		prospectSection := section(line)
		if len(prospectSection) > 0 {
			sctn = strings.ToLower(prospectSection)
			if sctn == "changelog" {
				// Changelog is the last section, no more key value pairs to expect
				break
			}
			log.Printf("new section %q\n", sctn)
			continue
		}
		k, v := keyValue(line)
		if k != "" {
			k = fmt.Sprintf("%s_%s", sctn, strings.ToLower(k))
			log.Printf("metadata: %q=%q\n", k, v)
			m[k] = v
		}
	}
	err := scanner.Err()
	return m, err
}

// return section name or empty string if no section
func section(line string) string {
	if strings.HasPrefix(line, sectionPrefix) && strings.HasSuffix(line, sectionSuffix) {
		line = strings.TrimPrefix(line, sectionPrefix)
		line = strings.TrimSuffix(line, sectionSuffix)
		return strings.ToLower(line)
	}
	return ""
}

// trim whitespace from beginning of line
func trim(line string) string {
	return strings.TrimSpace(line)
}
