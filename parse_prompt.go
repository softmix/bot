package main

import (
	"regexp"
	"strconv"
	"strings"
)

func ParsePrompt(prompt string) txt2img_request {
	var request txt2img_request
	hrForced := false

	var re = regexp.MustCompile(`(\S+):(\S+)`)
	matches := re.FindAllStringSubmatch(prompt, -1)
	for _, match := range matches {
		switch match[1] {
		case "cfg":
			if v, err := strconv.ParseFloat(match[2], 32); err == nil {
				request.CfgScale = clampf(float32(v), 1, 30)
			}
		case "count":
			if v, err := strconv.ParseInt(match[2], 10, 32); err == nil {
				request.NIter = clamp(int(v), 1, 9)
			}
		case "h":
			if v, err := strconv.ParseInt(match[2], 10, 32); err == nil {
				request.Height = clamp((int(v)+64-1)&-64, 512, 2048)
			}
		case "w":
			if v, err := strconv.ParseInt(match[2], 10, 32); err == nil {
				request.Width = clamp((int(v)+64-1)&-64, 512, 2048)
			}
		case "hr":
			if v, err := strconv.ParseBool(match[2]); err == nil {
				hrForced = true
				request.EnableHR = v
			}
		case "steps":
			if v, err := strconv.ParseInt(match[2], 10, 32); err == nil {
				request.Steps = clamp((int(v)+64-1)&-64, 1, 150)
			}
		}
		prompt = strings.Replace(prompt, match[0], "", 1)
	}

	if (request.Width >= 1024 || request.Height >= 1024) && !hrForced {
		request.EnableHR = true
	}

	if request.EnableHR {
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
