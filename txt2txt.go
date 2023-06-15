package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	"maunium.net/go/mautrix/event"
)

type Txt2txt struct {
	aiCharacter AICharacter
	Histories   map[string]History
}

type AICharacter struct {
	name         string
	instructions string
}

type RequestData struct {
	UserInput              string   `json:"user_input"`
	History                History  `json:"history"`
	Mode                   string   `json:"mode"`
	Character              string   `json:"character"`
	InstructionTemplate    string   `json:"instruction_template"`
	YourName               string   `json:"your_name"`
	Regenerate             bool     `json:"regenerate"`
	Continue               bool     `json:"_continue"`
	StopAtNewline          bool     `json:"stop_at_newline"`
	ChatPromptSize         int      `json:"chat_prompt_size"`
	ChatGenerationAttempts int      `json:"chat_generation_attempts"`
	ChatInstructCommand    string   `json:"chat-instruct_command"`
	MaxNewTokens           int      `json:"max_new_tokens"`
	DoSample               bool     `json:"do_sample"`
	Temperature            float64  `json:"temperature"`
	TopP                   float64  `json:"top_p"`
	TypicalP               float64  `json:"typical_p"`
	EpsilonCutoff          int      `json:"epsilon_cutoff"`
	EtaCutoff              int      `json:"eta_cutoff"`
	Tfs                    int      `json:"tfs"`
	TopA                   int      `json:"top_a"`
	RepetitionPenalty      float64  `json:"repetition_penalty"`
	TopK                   int      `json:"top_k"`
	MinLength              int      `json:"min_length"`
	NoRepeatNgramSize      int      `json:"no_repeat_ngram_size"`
	NumBeams               int      `json:"num_beams"`
	PenaltyAlpha           int      `json:"penalty_alpha"`
	LengthPenalty          int      `json:"length_penalty"`
	EarlyStopping          bool     `json:"early_stopping"`
	MirostatMode           int      `json:"mirostat_mode"`
	MirostatTau            int      `json:"mirostat_tau"`
	MirostatEta            float64  `json:"mirostat_eta"`
	Seed                   int      `json:"seed"`
	AddBOSToken            bool     `json:"add_bos_token"`
	TruncationLength       int      `json:"truncation_length"`
	BanEOSToken            bool     `json:"ban_eos_token"`
	SkipSpecialTokens      bool     `json:"skip_special_tokens"`
	StoppingStrings        []string `json:"stopping_strings"`
}

type History struct {
	Internal [][]string `json:"internal"`
	Visible  [][]string `json:"visible"`
}

type IncomingData struct {
	Event   string  `json:"event"`
	History History `json:"history,omitempty"`
}

func dataForPrompt(username, user_input string, history History) RequestData {
	return RequestData{
		UserInput: user_input,
		History:   history,

		Mode:                "chat",
		Character:           Bot.txt2txt.aiCharacter.name,
		InstructionTemplate: "None",
		YourName:            username,

		Regenerate:             false,
		Continue:               false,
		StopAtNewline:          false,
		ChatPromptSize:         2048,
		ChatGenerationAttempts: 1,
		ChatInstructCommand:    "",

		MaxNewTokens:      250,
		DoSample:          true,
		Temperature:       1.0,
		TopP:              0.9,
		TypicalP:          1,
		EpsilonCutoff:     0,
		EtaCutoff:         0,
		Tfs:               1,
		TopA:              0,
		RepetitionPenalty: 1.1,
		TopK:              0,
		MinLength:         0,
		NoRepeatNgramSize: 0,
		NumBeams:          1,
		PenaltyAlpha:      0,
		LengthPenalty:     1,
		EarlyStopping:     false,
		MirostatMode:      0,
		MirostatTau:       5,
		MirostatEta:       0.1,
		Seed:              -1,
		AddBOSToken:       true,
		TruncationLength:  2048,
		BanEOSToken:       false,
		SkipSpecialTokens: true,
		StoppingStrings:   []string{},
	}
}

func NewTxt2txt() *Txt2txt {
	instructions_body, err := ioutil.ReadFile("prompts_instructions.md")
	if err != nil {
		log.Fatal("Couldn't read prompts_instructions.md")
	}

	return &Txt2txt{
		aiCharacter: AICharacter{
			name:         "Neuro-sama", // TODO
			instructions: string(instructions_body),
		},
		Histories: make(map[string]History),
	}
}

func (b *Txt2txt) SaveHistories() error {
	data, err := json.MarshalIndent(b.Histories, "", "  ")
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(Bot.configuration.Txt2TxtHistoryFile, data, 0644)
	if err != nil {
		return err
	}

	return nil
}

func (b *Txt2txt) LoadHistories() error {
	data, err := ioutil.ReadFile(Bot.configuration.Txt2TxtHistoryFile)
	if err != nil {
		if os.IsNotExist(err) {
			b.Histories = map[string]History{}
			b.SaveHistories()
			return nil
		}
		return err
	}
	err = json.Unmarshal(data, &b.Histories)
	if err != nil {
		return err
	}
	return nil
}

func (b *Txt2txt) GetPredictionForPrompt(event *event.Event, prompt string) (string, error) {
	history := b.Histories[string(event.RoomID)]
	if len(history.Visible) == 0 {
		history.Visible = [][]string{}
	}
	if len(history.Internal) == 0 {
		history.Internal = [][]string{}
	}

	username, err := Bot.client.GetDisplayName(event.Sender)
	if err != nil {
		return prompt, err
	}
	reply, err := run(dataForPrompt(username.DisplayName, prompt, history))
	if err != nil {
		fmt.Println("Error:", err)
		return prompt, err
	}

	if len(reply.Visible[0]) > 0 {
		b.Histories[string(event.RoomID)] = reply
		b.SaveHistories()
	}

	log.Debug("Bot response", reply)
	return reply.Visible[len(reply.Visible)-1][1], err
}

func run(requestData RequestData) (History, error) {
	conn, _, err := websocket.DefaultDialer.Dial(Bot.configuration.Txt2TxtAPIURL, nil)
	if err != nil {
		return requestData.History, err
	}
	defer conn.Close()

	messageBytes, err := json.Marshal(requestData)
	if err != nil {
		return requestData.History, err
	}

	log.Debug("sending:", string(messageBytes))
	err = conn.WriteMessage(websocket.TextMessage, messageBytes)
	if err != nil {
		return requestData.History, err
	}

	var incomingData IncomingData
	var result History
	curLen := 0
processLoop:
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			return requestData.History, err
		}
		log.Debug("received:", string(message))

		err = json.Unmarshal(message, &incomingData)
		if err != nil {
			return requestData.History, err
		}

		switch incomingData.Event {
		case "text_stream":
			curMessage := incomingData.History.Visible[len(incomingData.History.Visible)-1][1][curLen:]
			curLen += len(curMessage)
			fmt.Print(curMessage)
			result = incomingData.History
		case "stream_end":
			break processLoop
		}
	}
	return result, nil
}

func randomHash() string {
	chars := "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, 9)
	for i := range result {
		index, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		result[i] = chars[index.Int64()]
	}
	return string(result)
}
