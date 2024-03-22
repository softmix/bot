package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"os"
	"strings"

	"github.com/rs/zerolog/log"
	"maunium.net/go/mautrix/event"
)

type Txt2txt struct {
	aiCharacter AICharacter
	Histories   map[string][]Message
}

type AICharacter struct {
	name         string
	instructions string
}

type RequestData struct {
	Messages  []Message `json:"messages"`
	Mode      string    `json:"mode"`
	Character string    `json:"character,omitempty"`
	Stream    bool      `json:"stream"`
}

type IncomingData struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int    `json:"created"`
	Model   string `json:"model"`
	Choices []Resp `json:"choices"`
	Usage   Usage  `json:"usage"`
}

type Resp struct {
	Index        int     `json:"index"`
	FinishReason *string `json:"finish_reason"`
	Delta        Message `json:"delta"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func dataForPrompt(username, user_input string, history []Message) RequestData {
	return RequestData{
		Messages: append(history, Message{
			Role:    "user",
			Content: user_input,
		}),
		Mode:   "chat",
		Stream: true,
		//Character: Bot.txt2txt.aiCharacter.name,
	}
}

func NewTxt2txt() *Txt2txt {
	instructions_body, err := os.ReadFile("prompts_instructions.md")
	if err != nil {
		log.Fatal().Msg("Couldn't read prompts_instructions.md")
	}

	return &Txt2txt{
		aiCharacter: AICharacter{
			name:         "TavernAI-Gray", // TODO
			instructions: string(instructions_body),
		},
		Histories: make(map[string][]Message),
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
			b.Histories = map[string][]Message{}
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
	if len(history) == 0 {
		history = []Message{}
	}

	username, err := Bot.client.GetDisplayName(context.Background(), event.Sender)
	if err != nil {
		return prompt, err
	}
	reply, err := run(dataForPrompt(username.DisplayName, prompt, history))
	if err != nil {
		fmt.Println("Error:", err)
		return prompt, err
	}

	if len(reply) > 0 {
		b.Histories[string(event.RoomID)] = reply
		b.SaveHistories()
	}

	log.Debug().Msgf("Bot response: %s", reply)
	return reply[len(reply)-1].Content, nil
}

func run(requestData RequestData) ([]Message, error) {
	// Marshal the request data to JSON
	requestDataBytes, err := json.Marshal(requestData)
	if err != nil {
		return requestData.Messages, err
	}

	// Create a new request
	req, err := http.NewRequest("POST", Bot.configuration.Txt2TxtAPIURL, bytes.NewBuffer(requestDataBytes))
	if err != nil {
		return requestData.Messages, err
	}
	req.Header.Set("Content-Type", "application/json")

	// Execute the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return requestData.Messages, err
	}
	defer resp.Body.Close()

	// Ensure we only accept a 200 OK response indicating that the SSE stream is established
	if resp.StatusCode != http.StatusOK {
		return requestData.Messages, fmt.Errorf("received non-200 status code: %d", resp)
	}

	// Create a buffered reader for the response body to read line by line
	reader := bufio.NewReader(resp.Body)
	var result []Message

	var currentMessageContent string
processLoop:
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err.Error() == "unexpected EOF" {
				log.Error().Err(err).Msg("Unexpected EOF")
				break
			}
			log.Error().Err(err).Msg("Error reading line")
			return requestData.Messages, err
		}

		// Process only lines starting with "data: ", which contains the actual message
		if strings.HasPrefix(string(line), "data: ") {
			var incomingData IncomingData
			dataBytes := line[5:] // Remove "data: " prefix and trim the line
			dataBytes = bytes.TrimSpace(dataBytes)

			err = json.Unmarshal(dataBytes, &incomingData)
			if err != nil {
				return requestData.Messages, err
			}

			currentMessageContent += incomingData.Choices[0].Delta.Content

			fmt.Print(incomingData.Choices[0].Delta.Content)

			if incomingData.Choices[0].FinishReason != nil {
				result = append(requestData.Messages, Message{
					Role:    incomingData.Choices[0].Delta.Role,
					Content: currentMessageContent,
				})
				currentMessageContent = ""
				break processLoop
			}
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
