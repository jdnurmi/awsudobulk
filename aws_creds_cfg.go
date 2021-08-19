package main

import (
	"gopkg.in/ini.v1"

	"os"
	"path/filepath"
	"strings"
)

type CredentialsFile struct {
	creds  *ini.File
	config *ini.File
}

func (cf *CredentialsFile) loadDefaultCredentials() (err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	if err == nil {
		cf.config, err = ini.Load(filepath.Join(home, ".aws", "config"))
	}
	cf.creds, err = ini.Load(filepath.Join(home, ".aws", "credentials"))
	return
}

// Load the default credentials file and return it as an ini object.
func LoadDefaultCredentials() (cf *CredentialsFile, err error) {
	cf = &CredentialsFile{}
	err = cf.loadDefaultCredentials()
	return
}

// Iterate through each section, calling fn with the section name
// and a list of sections.  If a section has a source_profile,
// that section is PREPENDED to the list (recursively).
func (cf *CredentialsFile) EachCredentialSection(fn func(string, ...*ini.Section) error) (err error) {
	for _, name := range cf.creds.SectionStrings() {
		section := cf.creds.Section(name)
		sections := []*ini.Section{section}

		for section.HasKey("source_profile") {
			pkey, err := section.GetKey("source_profile")
			if err != nil {
				break
			}
			section = cf.creds.Section(pkey.String())
			sections = append([]*ini.Section{}, append([]*ini.Section{section}, sections...)...)
		}
		if err == nil {
			err = fn(name, sections...)
		}
		if err != nil {
			break
		}
	}
	return
}

// Iterate through each section, calling fn with the section name
// and a list of sections.  If a section has a source_profile,
// that section is PREPENDED to the list (recursively).
func (cf *CredentialsFile) EachProfileSection(fn func(string, ...*ini.Section) error) (err error) {
	for _, name := range cf.config.SectionStrings() {
		if !strings.HasPrefix(name, "profile ") {
			continue
		}

		section := cf.config.Section(name)
		name = strings.TrimSpace(name[8:])
		sections := []*ini.Section{section}

		for section.HasKey("source_profile") {
			pkey, err := section.GetKey("source_profile")
			if err != nil {
				break
			}
			section = cf.config.Section(pkey.String())
			sections = append([]*ini.Section{}, append([]*ini.Section{section}, sections...)...)
		}
		if err == nil {
			err = fn(name, sections...)
		}
		if err != nil {
			break
		}
	}
	return
}

// Recurse through a list of sections looking for a Key(string)
// Last-one-set wins
func GetValueString(key string, sections ...*ini.Section) (s string, ok bool) {
	for i := range sections {
		key, err := sections[i].GetKey(key)
		if err != nil {
			continue
		}
		ok = true
		s = key.String()
	}
	return
}
