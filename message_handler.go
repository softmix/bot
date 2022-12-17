package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"net/http"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	log "github.com/sirupsen/logrus"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/crypto/attachment"
	mevent "maunium.net/go/mautrix/event"
)

func HandleMessage(source mautrix.EventSource, event *mevent.Event) {
	if event.Sender.String() == Bot.configuration.Username {
		log.Infof("Event %s is from us, so not going to respond.", event.ID)
		return
	}

	log.Info("Parsed content:", event.Content.Parsed)
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
			if image, err := getImageForPrompt(event, prompt); err != nil {
				sendReaction(event, "❌")
			} else {
				sendImage(event, "image.png", image)
				sendReaction(event, "✔️")
			}
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

func sendImage(event *mevent.Event, filename string, imageBytes []byte) {
	var file *attachment.EncryptedFile
	cfg, _, _ := image.DecodeConfig(bytes.NewReader(imageBytes))

	content := &mevent.MessageEventContent{
		MsgType: mevent.MsgImage,
		Body:    filename,
		Info: &mevent.FileInfo{
			Height:   cfg.Height,
			MimeType: http.DetectContentType(imageBytes),
			Width:    cfg.Height,
			Size:     len(imageBytes),
		},

		RelatesTo: &mevent.RelatesTo{
			EventID: event.ID,
			InReplyTo: &mevent.InReplyTo{
				EventID: event.ID,
			},
		},
	}

	uploadMime := content.Info.MimeType
	if Bot.stateStore.IsEncrypted(event.RoomID) {
		file = attachment.NewEncryptedFile()
		file.EncryptInPlace(imageBytes)
		uploadMime = "application/octet-stream"
	}

	req := mautrix.ReqUploadMedia{
		ContentBytes: imageBytes,
		ContentType:  uploadMime,
	}

	upload, err := Bot.client.UploadMedia(req)
	if err != nil {
		log.Error("Failed to upload media", err)
	}

	if file != nil {
		content.File = &mevent.EncryptedFileInfo{
			EncryptedFile: *file,
			URL:           upload.ContentURI.CUString(),
		}
	} else {
		content.URL = upload.ContentURI.CUString()
	}
	SendMessage(event.RoomID, content)
}

func getImageForPrompt(event *mevent.Event, prompt string) ([]byte, error) {
	req_body := ParsePrompt(prompt)

	json_body, err := json.Marshal(req_body)
	if err != nil {
		log.Error("Failed to marshal fields to JSON", err)
		return nil, err
	}

	resp, err := http.Post(Bot.configuration.SDAPIURL, "application/json", bytes.NewBuffer(json_body))
	if err != nil {
		log.Error("Failed to POST to SD API", err)
		return nil, err
	}
	defer resp.Body.Close()

	fmt.Println("response Status:", resp.Status)
	fmt.Println("response Headers:", resp.Header)

	var res txt2img_response
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		log.Error("Couldn't decode the response", err)
		return nil, err
	}
	encoded_image := res.Images[0]
	//for _, encoded_image := range res.Images {
	image, err := base64.StdEncoding.DecodeString(encoded_image)
	if err != nil {
		log.Error("Failed to decode the image", err)
		//continue
		return nil, err
	}
	//}
	return image, err
}
