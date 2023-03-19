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
			args: args{"some happy prompt h:512 w:600 ### not this tho cfg:12.3 ds:.6 hr:1 sampler:ddim"},
			want: txt2img_request{
				HRUpscaler:        "Latent",
				Prompt:            "some happy prompt",
				NegativePrompt:    "not this tho",
				CfgScale:          12.3,
				DenoisingStrength: 0.6,
				EnableHR:          true,
				SamplerName:       "DDIM",
				Height:            512,
				Width:             640, // rounded to multiple of 64
			},
		},
		{
			name: "a prompt with invalid values",
			args: args{"w:g cfg:31 h:0 w:3000 steps:1000 count:10 sampler:foobar ds:2"},
			want: txt2img_request{
				// sampler ignored
				DenoisingStrength: 1, CfgScale: 30, Width: 768, Height: 64, NIter: 9, Steps: 150, // clamped
			},
		},
		{
			name: "a prompt with weights",
			args: args{"w:704 a (high:1.4) (low:0.3) prompt h:704"},
			want: txt2img_request{
				Prompt: "a (high:1.4) (low:0.3) prompt", //hasn't removed h: or w:
				Width:  704,                             // hasn't tried to set it from low:
				Height: 704,                             // hasn't tried to set it from high:
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := json.Marshal(ParsePromptForTxt2Img(tt.args.prompt))
			want, _ := json.Marshal(tt.want)
			if !reflect.DeepEqual(string(got), string(want)) {
				log.Errorf("ParsePrompt() = %v, want %v", string(got), string(want))
			}
		})
	}
}
