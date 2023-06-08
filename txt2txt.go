package main

import (
	"bot/tools"
	"crypto/rand"
	"encoding/json"
	"errors"
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
	Histories   map[string]string
}

type AICharacter struct {
	instructions string
}

func (b *Txt2txt) contextualPrompt(roomID string, prompt string) string {
	history := Bot.txt2txt.Histories[roomID]
	if history != "" {
		return strings.Join([]string{
			strings.TrimSpace(history),
			"{{user}}: " + prompt,
			"{{character}}: ",
			//"Thought: Do I need to use a tool?",
		}, "\n\n")
	} else {
		return strings.Join([]string{
			initialPromt(),
			//toolPrompt(),
			"{{user}}: " + prompt,
			"{{character}}: ",
			//"Thought: Do I need to use a tool?",
		}, "\n\n")

	}
}

func initialPromt() string {
	if len(os.Args) > 1 {
		data, err := ioutil.ReadFile(os.Args[1])
		if err != nil {
			log.Error(err)
			return ""
		}
		return string(data)
	} else {
		return Bot.txt2txt.aiCharacter.instructions
	}
}

func runTool(toolName string, input string) (string, error) {
	tool, found := tools.AvailableTools[toolName]
	if !found {
		return "", errors.New("Unknown tool")
	}

	return tool.Run(input)
}

func toolPrompt() string {
	var toolNames []string
	for _, tool := range tools.AvailableTools {
		toolNames = append(toolNames, tool.Name())
	}

	toolNamesAndDescriptions := ""
	for _, tool := range tools.AvailableTools {
		toolNamesAndDescriptions += "- " + tool.Name() + ": " + tool.Description() + "\n"
	}

	return strings.Join([]string{
		"{{character}} has access to the following tools:",
		"",
		"",
		toolNamesAndDescriptions,
		"",
		"",
		"To use a tool, please use the following format:",
		"",
		"```",
		"Thought: Do I need to use a tool? Yes",
		"Action: the action to take, should be one of [" + strings.Join(toolNames, ", ") + "]",
		"Action Input: the input to the action",
		"Observation: the result of the action",
		"```",
		"",
		"When you have a response to say to the Human, or if you do not need to use a tool, you MUST use the format:",
		"",
		"```",
		"Thought: Do I need to use a tool? No",
		"{{character}}: [your response here]",
		"```",
	}, "\n")
}

func dataForPrompt(prompt string) []interface{} {
	params := map[string]interface{}{
		"max_new_tokens":             250,
		"do_sample":                  true,
		"temperature":                0.9,
		"top_p":                      0.9,
		"typical_p":                  1,
		"repetition_penalty":         1.05,
		"encoder_repetition_penalty": 1.0,
		"top_k":                      0,
		"min_length":                 0,
		"no_repeat_ngram_size":       0,
		"num_beams":                  1,
		"penalty_alpha":              0,
		"length_penalty":             1,
		"early_stopping":             false,
		"seed":                       -1,
		"add_bos_token":              true,
		"truncation_length":          8192,
		"custom_stopping_strings":    []string{},
		"ban_eos_token":              false,
	}
	return []interface{}{prompt, params}
}

func NewTxt2txt() *Txt2txt {
	instructions_body, err := ioutil.ReadFile("prompts_instructions.md")
	if err != nil {
		log.Fatal("Couldn't read prompts_instructions.md")
	}

	return &Txt2txt{
		aiCharacter: AICharacter{
			instructions: string(instructions_body),
		},
		Histories: make(map[string]string),
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

func (b *Txt2txt) GetPredictionForPrompt(event *event.Event, prompt string) (string, error) {
	contextualPrompt := strings.TrimSpace(b.contextualPrompt(string(event.RoomID), prompt)) + " "

	reply, err := run(contextualPrompt)
	if err != nil {
		fmt.Println("Error:", err)
		return prompt, err
	}

	b.Histories[string(event.RoomID)] = reply

	if len(reply) > len(contextualPrompt) {
		reply = strings.TrimSpace(reply[len(contextualPrompt):])
	} else {
		reply = ""
	}

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

	var lines []string
	var processStartTime time.Time

processLoop:
	for {
		var content map[string]interface{}

		err := conn.ReadJSON(&content)
		if err != nil {
			log.Error("Error reading JSON", err)
			break
		}
		log.Info(content)

		msg, ok := content["msg"].(string)
		if !ok {
			continue
		}

		fn_index := 43

		switch msg {
		case "send_hash":
			err := conn.WriteJSON(map[string]interface{}{
				"session_hash": session,
				"fn_index":     fn_index,
			})
			if err != nil {
				fmt.Println("Error sending JSON:", err)
				return "", err
			}
		case "estimation":
			continue
		case "send_data":
			dataJson, err := json.Marshal(dataForPrompt(prompt))
			if err != nil {
				panic(err)
			}
			log.Info("dataJson: ", string(dataJson))
			err = conn.WriteJSON(map[string]interface{}{
				"session_hash": session,
				"fn_index":     fn_index,
				"data":         []interface{}{string(dataJson)},
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

					log.Info("\n", response)
					if ok {
						lines = strings.Split(response, "\n")
						last_line := lines[len(lines)-1]
						if strings.HasPrefix(last_line, "### Human:") || strings.HasPrefix(last_line, "You:") || strings.HasPrefix(last_line, "{{user}}:") {
							lines = lines[:len(lines)-1]
							break processLoop
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
	return strings.Join(lines, "\n"), nil
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
