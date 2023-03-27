package main

import (
	"bytes"
	"image"
	"net/http"
	"os"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	log "github.com/sirupsen/logrus"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/crypto/attachment"
	mevent "maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/format"
)

func HandleMessage(source mautrix.EventSource, event *mevent.Event) {
	if event.Sender.String() == Bot.configuration.Username {
		log.Infof("Event %s is from us, so not going to respond.", event.ID)
		return
	}

	log.Info("Parsed content:", event.Content.Parsed)
	content := event.Content.AsMessage()
	content.RemoveReplyFallback()
	body := content.Body
	switch content.MsgType {
	case mevent.MsgText, mevent.MsgNotice:
		if body == "ping" {
			sendMessage(event, "pong")
			return
		}

		if body == "yay" {
			sendReaction(event, "🎉")
			return
		}

		if body == "!gen help" {
			if help, err := os.ReadFile("./help.md"); err == nil {
				sendMarkdown(event, string(help))
				return
			}
		}

		if body == "!forget" {
			Bot.txt2txt.Histories[string(event.RoomID)] = &History{Messages: make([]Message, 0)}
			err := Bot.txt2txt.SaveHistories()
			if err != nil {
				log.Error("Failed to save history", err)
				sendMessage(event, "Couldn't forget")
				return
			}
			sendReaction(event, "🤯")
		}

		if strings.HasPrefix(body, Bot.configuration.DisplayName+": ") {
			prompt := strings.TrimPrefix(body, Bot.configuration.DisplayName+": ")
			if len(prompt) == 0 {
				break
			}

			Bot.client.UserTyping(event.RoomID, true, 10*time.Second)

			if reply, err := Bot.txt2txt.GetPredictionForPrompt(event, prompt); err != nil {
				sendReaction(event, "❌")
			} else {
				sendMessage(event, reply)
			}

			Bot.client.UserTyping(event.RoomID, false, 0)

			return
		}

		if strings.HasPrefix(body, "!gen ") {
			prompt := strings.TrimPrefix(body, "!gen ")
			if len(prompt) == 0 {
				break
			}
			sendReaction(event, "👌")
			if image, err := getImageForPrompt(event, prompt); err != nil {
				sendMessage(event, "i'm sorry dave, i'm afraid i can't do that")
				sendReaction(event, "❌")
			} else {
				sendImage(event, "image.jpg", image)
				sendReaction(event, "✔️")
			}
			return
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

func sendMarkdown(event *mevent.Event, text string) {
	content := format.RenderMarkdown(text, true, false)
	SendMessage(event.RoomID, &content)
}

func sendMessage(event *mevent.Event, text string) {
	content := mevent.MessageEventContent{
		MsgType: mevent.MsgText,
		Body:    text,

		RelatesTo: &mevent.RelatesTo{
			EventID: event.ID,
			InReplyTo: &mevent.InReplyTo{
				EventID: event.ID,
			},
		},
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
