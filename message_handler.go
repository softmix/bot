package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	log "github.com/sirupsen/logrus"
	"maunium.net/go/mautrix"
	mevent "maunium.net/go/mautrix/event"
)

func HandleMessage(source mautrix.EventSource, event *mevent.Event) {
	if event.Sender.String() == Bot.configuration.Username {
		log.Infof("Event %s is from us, so not going to respond.", event.ID)
		return
	}

	log.Infof("Parsed content:", event.Content.Parsed)
	content := event.Content.AsMessage()
	body := content.Body
	switch content.MsgType {
	case mevent.MsgText, mevent.MsgNotice:
		if body == "ping" {
			sendMessage(event, "pong")
		}

		if body == "yay" {
			sendReaction(event, "🎉")
		}

		if strings.HasPrefix(body, "!gen ") {
			prompt := strings.TrimPrefix(body, "!gen ")
			if len(prompt) == 0 {
				break
			}
			sendReaction(event, "👌")
			sendImageForPrompt(event, prompt)
			sendReaction(event, "✔️")
		}
		break
	case mevent.MsgEmote:
		break
	case mevent.MsgAudio, mevent.MsgFile, mevent.MsgImage, mevent.MsgVideo:
		break
	}
}

func sendReaction(event *mevent.Event, reaction string) {
	Bot.client.SendReaction(event.RoomID, event.ID, reaction)
}

func sendMessage(event *mevent.Event, text string) {
	content := mevent.MessageEventContent{
		MsgType: mevent.MsgText,
		Body:    text,
	}
	SendMessage(event.RoomID, &content)
}

func sendImage(event *mevent.Event, filename string, image []byte) {
	upload, err := Bot.client.UploadMedia(mautrix.ReqUploadMedia{
		Content:       bytes.NewReader(image),
		ContentLength: int64(len(image)),
		ContentType:   "image/png",
	})
	if err != nil {
		log.Errorf("Got an error when uploading media.", err)
		return
	}
	content := mevent.MessageEventContent{
		MsgType: mevent.MsgImage,
		Body:    filename,
		URL:     upload.ContentURI.CUString(),
	}
	SendMessage(event.RoomID, &content)
}

func sendImageForPrompt(event *mevent.Event, prompt string) {
	var req_body txt2img_request
	req_body.Prompt = prompt

	json_body, err := json.Marshal(req_body)
	if err != nil {
		log.Errorf("Failed to marshal fields to JSON, %w", err)
		return
	}

	resp, err := http.Post(Bot.configuration.SDAPIURL, "application/json", bytes.NewBuffer(json_body))
	if err != nil {
		log.Errorf("Failed to POST to SD API, %w", err)
		return
	}
	defer resp.Body.Close()

	fmt.Println("response Status:", resp.Status)
	fmt.Println("response Headers:", resp.Header)

	var res txt2img_response
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		log.Errorf("Couldn't decode the response, %w", err)
		return
	}
	for _, encoded_image := range res.Images {
		image, err := base64.StdEncoding.DecodeString(encoded_image)
		if err != nil {
			log.Errorf("Failed to decode the image", err)
			continue
		}
		sendImage(event, "image.png", image)
	}
}
