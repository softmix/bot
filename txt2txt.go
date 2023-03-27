package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"maunium.net/go/mautrix/event"
)

type Txt2txt struct {
	aiCharacter AICharacter
	Histories   map[string]*History
}

type AICharacter struct {
	Name           string
	Persona        string
	Greeting       string
	WorldScenario  string
	DialogueSample string
}

type txt2txt_request struct {
	Data []interface{} `json:"data"`
}

type txt2txt_response struct {
	Data     []string `json:"data"`
	Duration float64  `json:"duration"`
}

type Message struct {
	Sender  string
	Content string
}

type History struct {
	Messages []Message
}

func (b *Txt2txt) parsePromptForTxt2Txt(contextIdentifier string, prompt string) string {
	aiPreprompt := ""
	aiPreprompt += b.aiCharacter.Name + "'s persona: " + b.aiCharacter.Persona + "\n"
	aiPreprompt += "The world scenario: " + b.aiCharacter.WorldScenario + "\n"
	aiPreprompt += "<START>\n" + b.aiCharacter.DialogueSample + "\n"

	aiHistory := ""
	for _, m := range b.GetHistory(contextIdentifier).Messages {
		aiHistory += m.Sender + ": " + m.Content + "\n"
	}

	contextualPrompt := aiPreprompt + aiHistory + "You: " + prompt + "\n" + b.aiCharacter.Name + ":"

	return contextualPrompt
}

func requestForPrompt(prompt string) txt2txt_request {
	var request txt2txt_request
	data := []interface{}{
		prompt, // Prompt
		200,    // MaxNewTokens
		true,   // DoSample
		0.5,    // Temperature
		0.9,    // TopP
		1,      // TypicalP
		1.05,   // RepetitionPenalty
		1.0,    // EncoderRepetitionPenalty
		0,      // TopK
		0,      // MinLength
		0,      // NoRepeatNgramSize
		1,      // NumBeams
		0,      // PenaltyAlpha
		1,      // LengthPenalty
		false,  // EarlyStopping
		-1,     // Seed
	}
	request.Data = data
	return request
}

func NewTxt2txt() *Txt2txt {
	return &Txt2txt{
		aiCharacter: AICharacter{
			Name:           "Bot",
			Persona:        "Bot is a helpful AI chatbot that always provides useful and detailed answers to User's requests and questions. Bot tries to be as informative and friendly as possible.",
			Greeting:       "Hello! I am Bot, your informative assistant. How may I help you today?",
			DialogueSample: "You: Hi. Can you help me with something?\nBot: Hello, this is Johm. How can I help?\nYou: Have you heard of the latest nuclear fusion experiment from South Korea? I heard their experiment got hotter than the sun.\nBot: Yes, I have heard about the experiment. Scientists in South Korea have managed to sustain a nuclear fusion reaction running at temperatures in excess of 100 million°C for 30 seconds for the first time and have finally been able to achieve a net energy gain when carrying out a nuclear fusion experiment. That's nearly seven times hotter than the core of the Sun, which has a temperature of 15 million degrees kelvins! That's exciting!\nYou: Wow! That's super interesting to know. Change of topic, I plan to change to the iPhone 14 this year.\nBot: I see. What makes you want to change to iPhone 14?\nYou: My phone right now is too old, so I want to upgrade.\nBot: That's always a good reason to upgrade. You should be able to save money by trading in your old phone for credit. I hope you enjoy your new phone when you upgrade.",
		},
		Histories: make(map[string]*History),
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

func (b *Txt2txt) AddMessage(channel string, message Message) {
	if b.Histories[channel] == nil {
		b.Histories[channel] = &History{Messages: make([]Message, 0)}
	}
	b.Histories[channel].Messages = append(b.Histories[channel].Messages, message)
}

func (b *Txt2txt) GetHistory(channel string) *History {
	if b.Histories[channel] == nil {
		b.Histories[channel] = &History{Messages: make([]Message, 0)}
	}
	return b.Histories[channel]
}

func (b *Txt2txt) PrintAllHistories() {
	log.Info(b.Histories)
}

func (b *Txt2txt) GetPredictionForPrompt(event *event.Event, prompt string) (string, error) {
	contextualPrompt := b.parsePromptForTxt2Txt(string(event.RoomID), prompt)

	req_body := requestForPrompt(contextualPrompt)

	json_body, err := json.Marshal(req_body)
	if err != nil {
		log.Error("Failed to marshal fields to JSON", err)
		return "", err
	}

	resp, err := http.Post(Bot.configuration.Txt2TxtAPIURL, "application/json", bytes.NewBuffer(json_body))
	if err != nil {
		log.Error("Failed to POST to LLaMA API", err)
		return "", err
	}
	defer resp.Body.Close()

	fmt.Println("response Status:", resp.Status)
	fmt.Println("response Headers:", resp.Header)

	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("Error reading request body: %v", err.Error())
		return "", err
	}
	fmt.Println("response Body:", string(buf))

	reader := ioutil.NopCloser(bytes.NewBuffer(buf))
	resp.Body = reader

	var res txt2txt_response
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		log.Error("Couldn't decode the response", err)
		return "", err
	}

	if len(res.Data) == 0 {
		return "", errors.New("No data in response")
	}

	reply := res.Data[0][len(contextualPrompt):]
	k := strings.Index(reply, "\nYou:")
	if k != -1 {
		reply = reply[:k]
	}

	k = strings.Index(reply, "\n"+b.aiCharacter.Name)
	if k != -1 {
		reply = reply[:k]
	}

	reply = strings.TrimSpace(reply)

	b.AddMessage(string(event.RoomID), Message{Sender: "You", Content: prompt})
	b.AddMessage(string(event.RoomID), Message{Sender: b.aiCharacter.Name, Content: reply})
	b.PrintAllHistories()

	log.Info("Bot response", reply)
	return reply, err
}
