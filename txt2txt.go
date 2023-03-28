package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
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

func dataForPrompt(prompt string) []interface{} {
	return []interface{}{
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

	reply, err := run(contextualPrompt)
	if err != nil {
		fmt.Println("Error:", err)
		return "", err
	}

	k := strings.Index(reply, "\n"+b.aiCharacter.Name)
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

func run(prompt string) (string, error) {
	session := randomHash()

	conn, _, err := websocket.DefaultDialer.Dial(Bot.configuration.Txt2TxtAPIURL, nil)
	if err != nil {
		return "", err
	}

	defer conn.Close()

	var finalReponse string
	var processStartTime time.Time

processLoop:
	for {
		var content map[string]interface{}

		err := conn.ReadJSON(&content)
		if err != nil {
			log.Error("Error reading JSON", err)
			break
		}

		msg, ok := content["msg"].(string)
		if !ok {
			continue
		}

		switch msg {
		case "send_hash":
			err := conn.WriteJSON(map[string]interface{}{
				"session_hash": session,
				"fn_index":     12,
			})
			if err != nil {
				fmt.Println("Error sending JSON:", err)
				return "", err
			}
		case "estimation":
			continue
		case "send_data":
			err := conn.WriteJSON(map[string]interface{}{
				"session_hash": session,
				"fn_index":     12,
				"data":         dataForPrompt(prompt),
			})
			if err != nil {
				fmt.Println("Error sending JSON:", err)
				return "", err
			}
		case "process_starts":
			processStartTime = time.Now()
			continue
		case "process_generating", "process_completed":
			output, ok := content["output"].(map[string]interface{})
			if ok {
				data, ok := output["data"].([]interface{})
				if ok && len(data) > 0 {
					response, ok := data[0].(string)
					if ok {
						response = response[len(prompt):]
						lines := strings.Split(response, "\n")
						foundYouPrefix := false
						for _, line := range lines {
							if strings.HasPrefix(line, "You:") {
								foundYouPrefix = true
								lines = lines[:len(lines)-1]
								finalReponse = strings.Join(lines, "\n")
								log.Info("Response generated in ", time.Since(processStartTime))
								break processLoop
							}
						}
						if !foundYouPrefix {
							finalReponse = strings.Join(lines, "\n")
						}
					}
				}
			}
			if msg == "process_completed" {
				break
			}
		}
	}

	log.Info("Response generated in ", time.Since(processStartTime))
	return finalReponse, nil
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
