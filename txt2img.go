package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
	mevent "maunium.net/go/mautrix/event"
)

type txt2img_request struct {
	EnableHR                          bool     `json:"enable_hr,omitempty"`
	DenoisingStrength                 float32  `json:"denoising_strength,omitempty"`
	FirstphaseWidth                   int      `json:"firstphase_width,omitempty"`
	FirstphaseHeight                  int      `json:"firstphase_height,omitempty"`
	HRScale                           float32  `json:"hr_scale,omitempty"`
	HRUpscaler                        string   `json:"hr_upscaler,omitempty"`
	HRSecondPassSteps                 int      `json:"hr_second_pass_steps,omitempty"`
	HRResizeX                         int      `json:"hr_resize_x,omitempty"`
	HRResizeY                         int      `json:"hr_resize_y,omitempty"`
	Prompt                            string   `json:"prompt,omitempty"`
	Styles                            []string `json:"styles,omitempty"`
	Seed                              int      `json:"seed,omitempty"`
	Subseed                           int      `json:"subseed,omitempty"`
	SubseedStrength                   float32  `json:"subseed_strength,omitempty"`
	SeedResizeFromH                   int      `json:"seed_resize_from_h,omitempty"`
	SeedResizeFromW                   int      `json:"seed_resize_from_w,omitempty"`
	SamplerName                       string   `json:"sampler_name,omitempty"`
	BatchSize                         int      `json:"batch_size,omitempty"`
	NIter                             int      `json:"n_iter,omitempty"`
	Steps                             int      `json:"steps,omitempty"`
	CfgScale                          float32  `json:"cfg_scale,omitempty"`
	Width                             int      `json:"width,omitempty"`
	Height                            int      `json:"height,omitempty"`
	RestoreFaces                      bool     `json:"restore_faces,omitempty"`
	Tiling                            bool     `json:"tiling,omitempty"`
	NegativePrompt                    string   `json:"negative_prompt,omitempty"`
	Eta                               float32  `json:"eta,omitempty"`
	SChurn                            float32  `json:"s_churn,omitempty"`
	STmax                             float32  `json:"s_tmax,omitempty"`
	STmin                             float32  `json:"s_tmin,omitempty"`
	SNoise                            float32  `json:"s_noise,omitempty"`
	OverrideSettings                  struct{} `json:"override_settings,omitempty"`
	OverrideSettingsRestoreAfterwards bool     `json:"override_settings_restore_afterwards,omitempty"`
	ScriptArgs                        []string `json:"script_args,omitempty"`
	SamplerIndex                      string   `json:"sampler_index,omitempty"`
	ScriptName                        string   `json:"script_name,omitempty"`
}

type txt2img_response struct {
	Images     []string `json:"images"`
	Parameters struct {
		EnableHr          bool        `json:"enable_hr"`
		DenoisingStrength float64     `json:"denoising_strength"`
		FirstphaseWidth   int         `json:"firstphase_width"`
		FirstphaseHeight  int         `json:"firstphase_height"`
		Prompt            string      `json:"prompt"`
		Styles            interface{} `json:"styles"`
		Seed              int         `json:"seed"`
		Subseed           int         `json:"subseed"`
		SubseedStrength   int         `json:"subseed_strength"`
		SeedResizeFromH   int         `json:"seed_resize_from_h"`
		SeedResizeFromW   int         `json:"seed_resize_from_w"`
		SamplerName       interface{} `json:"sampler_name"`
		BatchSize         int         `json:"batch_size"`
		NIter             int         `json:"n_iter"`
		Steps             int         `json:"steps"`
		CfgScale          float64     `json:"cfg_scale"`
		Width             int         `json:"width"`
		Height            int         `json:"height"`
		RestoreFaces      bool        `json:"restore_faces"`
		Tiling            bool        `json:"tiling"`
		NegativePrompt    interface{} `json:"negative_prompt"`
		Eta               interface{} `json:"eta"`
		SChurn            float64     `json:"s_churn"`
		STmax             interface{} `json:"s_tmax"`
		STmin             float64     `json:"s_tmin"`
		SNoise            float64     `json:"s_noise"`
		OverrideSettings  interface{} `json:"override_settings"`
		SamplerIndex      string      `json:"sampler_index"`
	} `json:"parameters"`
	Info string `json:"info"`
}

func ParsePromptForTxt2Img(prompt string) txt2img_request {
	var request txt2img_request
	forcedSettings := map[string]bool{}

	request.Width = 512
	request.Height = 512
	request.EnableHR = true
	request.SamplerName = "Restart"
	request.HRUpscaler = "4x_Valar_v1"
	request.Steps = 20

	var re = regexp.MustCompile(`(\S+):(\S+)`)
	matches := re.FindAllStringSubmatch(prompt, -1)
	for _, match := range matches {
		if ok := handleSetting(&request, &forcedSettings, match[1], match[2]); ok {
			replacement := regexp.MustCompile(`\s*` + match[0] + `\s*`)
			if found := replacement.FindString(prompt); found != "" {
				prompt = strings.Replace(prompt, found, " ", 1)
			}
		}
	}

	if !forcedSettings["ds"] && request.EnableHR {
		request.DenoisingStrength = 0.7
	}

	prompts := strings.Split(prompt, "###")
	request.Prompt = strings.TrimSpace(prompts[0])
	if len(prompts) == 2 {
		request.NegativePrompt = strings.TrimSpace(prompts[1])
	}

	return request
}

func getImageForPrompt(event *mevent.Event, prompt string) ([]byte, error) {
	req_body := ParsePromptForTxt2Img(prompt)

	json_body, err := json.Marshal(req_body)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal fields to JSON")
		return nil, err
	}

	resp, err := http.Post(Bot.configuration.Txt2ImgAPIURL, "application/json", bytes.NewBuffer(json_body))
	if err != nil {
		log.Error().Err(err).Msg("Failed to POST to SD API")
		return nil, err
	}
	defer resp.Body.Close()

	fmt.Println("response Status:", resp.Status)
	fmt.Println("response Headers:", resp.Header)

	var res txt2img_response
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		log.Error().Err(err).Msg("Couldn't decode the response")
		return nil, err
	}

	if len(res.Images) == 0 {
		return nil, errors.New("No images in response")
	}

	encoded_image := res.Images[0]
	//for _, encoded_image := range res.Images {
	image, err := base64.StdEncoding.DecodeString(encoded_image)
	if err != nil {
		log.Error().Err(err).Msg("Failed to decode the image")
		//continue
		return nil, err
	}
	//}
	return image, err
}

func handleSetting(request *txt2img_request, forcedSettings *map[string]bool, setting, value string) bool {
	supportedSamplers := map[string]string{
		"unipc":   "UniPC",
		"ddim":    "DDIM",
		"euler":   "Euler",
		"euler_a": "Euler a",
		"heun":    "Heun",
		"lms":     "LMS",
		"plms":    "PLMS",
	}

	supportedUpscalers := map[string]string{
		"latent":   "Latent",
		"none":     "None",
		"lanczos":  "Lanczos",
		"nearest":  "Nearest",
		"esrgan":   "ESRGAN_4x",
		"lollypop": "lollypop",
		"ldsr":     "LDSR",
	}

	switch setting {
	case "cfg":
		if v, err := strconv.ParseFloat(value, 32); err == nil {
			request.CfgScale = clampf(float32(v), 1, 30)
		}
		return true
	case "ds":
		if v, err := strconv.ParseFloat(value, 32); err == nil {
			(*forcedSettings)["ds"] = true
			request.DenoisingStrength = clampf(float32(v), 0, 1)
		}
		return true
	case "count":
		if v, err := strconv.ParseInt(value, 10, 32); err == nil {
			request.NIter = clamp(int(v), 1, 9)
		}
		return true
	case "h":
		if v, err := strconv.ParseInt(value, 10, 32); err == nil {
			request.Height = clamp((int(v)+64-1)&-64, 64, 768)
		}
		return true
	case "w":
		if v, err := strconv.ParseInt(value, 10, 32); err == nil {
			request.Width = clamp((int(v)+64-1)&-64, 64, 768)
		}
		return true
	case "hr":
		if v, err := strconv.ParseBool(value); err == nil {
			request.EnableHR = v
		}
		return true
	case "fr":
		if v, err := strconv.ParseBool(value); err == nil {
			request.RestoreFaces = v
		}
		return true
	case "scale":
		if v, err := strconv.ParseFloat(value, 32); err == nil {
			request.HRScale = clampf(float32(v), 1, 4)
		}
		return true
	case "upscaler":
		if sampler, ok := supportedUpscalers[value]; ok {
			(*forcedSettings)["upscaler"] = true
			request.HRUpscaler = sampler
		}
		return true
	case "steps":
		if v, err := strconv.ParseInt(value, 10, 32); err == nil {
			request.Steps = clamp(int(v), 4, 150)
		}
		return true
	case "sampler":
		if sampler, ok := supportedSamplers[value]; ok {
			request.SamplerName = sampler
		}
		return true
	}
	return false
}
