package main

import (
	"gopkg.in/yaml.v2"
)

type Configuration struct {
	SDAPIURL    string `yaml:"sdapi_url"`
	LLAMAAPIURL string `yaml:"llamaapi_url"`

	// Authentication
	Password   string `yaml:"password"`
	Username   string `yaml:"username"`
	Homeserver string `yaml:"homeserver"`

	// Bot
	DisplayName string `yaml:"display_name"`
	DebugRoom   string `yaml:"debug_room"`
}

func (c *Configuration) Parse(data []byte) error {
	return yaml.Unmarshal(data, c)
}
