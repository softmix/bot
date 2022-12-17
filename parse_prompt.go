package main

import (
	"regexp"
	"strconv"
	"strings"
)

func ParsePrompt(prompt string) txt2img_request {
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

	if !forcedSettings["hr"] && (request.Width >= 1024 || request.Height >= 1024) {
		request.EnableHR = true
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
			request.Height = clamp((int(v)+64-1)&-64, 512, 2048)
		}
		return true
	case "w":
		if v, err := strconv.ParseInt(value, 10, 32); err == nil {
			request.Width = clamp((int(v)+64-1)&-64, 512, 2048)
		}
		return true
	case "hr":
		if v, err := strconv.ParseBool(value); err == nil {
			(*forcedSettings)["hr"] = true
			request.EnableHR = v
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
