package main

import (
	"gopkg.in/yaml.v2"
)

type Configuration struct {
	Txt2ImgAPIURL      string `yaml:"txt2img_api_url"`
	Txt2TxtAPIURL      string `yaml:"txt2txt_api_url"`
	Txt2TxtHistoryFile string `yaml:"txt2txt_history_file"`

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
