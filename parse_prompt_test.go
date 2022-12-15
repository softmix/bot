package main

import (
	"encoding/json"
	"reflect"
	"testing"

	log "github.com/sirupsen/logrus"
)

func TestParsePrompt(t *testing.T) {
	type args struct {
		prompt string
	}
	tests := []struct {
		name string
		args args
		want txt2img_request
	}{
		{
			name: "a basic prompt with negatives and some configuration",
			args: args{"some happy prompt h:512 w:600 ### not this tho cfg:12.3"},
			want: txt2img_request{
				Prompt:         "some happy prompt",
				NegativePrompt: "not this tho",
				CfgScale:       12.3,
				Height:         512,
				Width:          640, // rounded to multiple of 64
			},
		},
		{
			name: "a prompt with invalid values",
			args: args{"w:g cfg:12.e h:0 w:3000"},
			want: txt2img_request{
				Width: 2048, Height: 512, // clamped
				EnableHR: true, // when a dimension is greater than 1024
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := json.Marshal(ParsePrompt(tt.args.prompt))
			want, _ := json.Marshal(tt.want)
			if !reflect.DeepEqual(string(got), string(want)) {
				log.Errorf("ParsePrompt() = %v, want %v", string(got), string(want))
			}
		})
	}
}
