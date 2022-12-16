package main

import (
	"bot/store"

	"maunium.net/go/mautrix"
	mcrypto "maunium.net/go/mautrix/crypto"
)

type txt2img_request struct {
	EnableHR          bool     `json:"enable_hr,omitempty"`
	DenoisingStrength float32  `json:"denoising_strength,omitempty"`
	FirstphaseWidth   int      `json:"firstphase_width,omitempty"`
	FirstphaseHeight  int      `json:"firstphase_height,omitempty"`
	Prompt            string   `json:"prompt,omitempty"`
	Styles            []string `json:"styles,omitempty"`
	Seed              int      `json:"seed,omitempty"`
	Subseed           int      `json:"subseed,omitempty"`
	SubseedStrength   float32  `json:"subseed_strength,omitempty"`
	SeedResizeFromH   int      `json:"seed_resize_from_h,omitempty"`
	SeedResizeFromW   int      `json:"seed_resize_from_w,omitempty"`
	SamplerName       string   `json:"sampler_name,omitempty"`
	BatchSize         int      `json:"batch_size,omitempty"`
	NIter             int      `json:"n_iter,omitempty"`
	Steps             int      `json:"steps,omitempty"`
	CfgScale          float32  `json:"cfg_scale,omitempty"`
	Width             int      `json:"width,omitempty"`
	Height            int      `json:"height,omitempty"`
	RestoreFaces      bool     `json:"restore_faces,omitempty"`
	Tiling            bool     `json:"tiling,omitempty"`
	NegativePrompt    string   `json:"negative_prompt,omitempty"`
	Eta               float32  `json:"eta,omitempty"`
	SChurn            float32  `json:"s_churn,omitempty"`
	STmax             float32  `json:"s_tmax,omitempty"`
	STmin             float32  `json:"s_tmin,omitempty"`
	SNoise            float32  `json:"s_noise,omitempty"`
	OverrideSettings  struct{} `json:"override_settings,omitempty"`
	SamplerIndex      string   `json:"sampler_index,omitempty"`
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
		OverrideSettings  struct {
		} `json:"override_settings"`
		SamplerIndex string `json:"sampler_index"`
	} `json:"parameters"`
	Info string `json:"info"`
}

type BotType struct {
	client        *mautrix.Client
	configuration Configuration
	olmMachine    *mcrypto.OlmMachine
	stateStore    *store.StateStore
}
