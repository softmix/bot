package main

import (
	"regexp"
	"strconv"
	"strings"
)

func ParsePromptForTxt2Txt(prompt string) llama_request {
	var request llama_request
	request.Data = append(request.Data, prompt)
	return request
}

func ParsePromptForTxt2Img(prompt string) txt2img_request {
	var request txt2img_request
	forcedSettings := map[string]bool{}

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

	if !forcedSettings["upscaler"] && request.EnableHR {
		request.HRUpscaler = "Latent"
	}

	prompts := strings.Split(prompt, "###")
	request.Prompt = strings.TrimSpace(prompts[0])
	if len(prompts) == 2 {
		request.NegativePrompt = strings.TrimSpace(prompts[1])
	}

	return request
}

func clamp(x, min, max int) int {
	switch {
	case x < min:
		return min
	case x > max:
		return max
	default:
		return x
	}
}

func clampf(x, min, max float32) float32 {
	switch {
	case x < min:
		return min
	case x > max:
		return max
	default:
		return x
	}
}

func handleSetting(request *txt2img_request, forcedSettings *map[string]bool, setting, value string) bool {
	supportedSamplers := map[string]string{
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
			request.Steps = clamp(int(v), 1, 150)
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
