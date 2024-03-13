package main

import (
	"bytes"
	"image"
	"net/http"
	"os"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog/log"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/crypto/attachment"
	mevent "maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/format"
)

func HandleMessage(source mautrix.EventSource, event *mevent.Event) {
	if event.Sender.String() == Bot.configuration.Username {
		log.Info().Msgf("Event %s is from us, so not going to respond.", event.ID)
		return
	}

	log.Info().Msgf("Parsed content: %s", event.Content.Parsed)
	content := event.Content.AsMessage()
	content.RemoveReplyFallback()
	body := content.Body
	switch content.MsgType {
	case mevent.MsgText, mevent.MsgNotice:
		if body == "ping" {
			sendReply(event, "pong")
			return
		}

		if body == "yay" {
			sendReaction(event, "🎉")
			return
		}

		if body == "!gen help" {
			if help, err := os.ReadFile("./help.md"); err == nil {
				sendMarkdown(event, string(help))
			}
			return
		}

		if body == "!forget" {
			delete(Bot.txt2txt.Histories, string(event.RoomID))
			err := Bot.txt2txt.SaveHistories()
			if err != nil {
				log.Error().Err(err).Msg("Failed to save history")
				sendReply(event, "Couldn't forget")
				return
			}
			sendReaction(event, "🤯")
			return
		}

		if strings.HasPrefix(body, "!gen ") {
			prompt := strings.TrimPrefix(body, "!gen ")
			if len(prompt) == 0 {
				break
			}
			sendReaction(event, "👌")
			if image, err := getImageForPrompt(event, prompt); err != nil {
				sendReply(event, "i'm sorry dave, i'm afraid i can't do that")
				sendReaction(event, "❌")
			} else {
				sendImage(event, "image.jpg", image)
				sendReaction(event, "✔️")
			}
			return
		}

		if strings.HasPrefix(body, Bot.configuration.DisplayName+": ") || len(Bot.stateStore.GetRoomMembers(event.RoomID)) == 2 {
			prompt := strings.TrimPrefix(body, Bot.configuration.DisplayName+": ")
			if len(prompt) == 0 {
				break
			}

			Bot.client.UserTyping(event.RoomID, true, 10*time.Second)

			reply, err := Bot.txt2txt.GetPredictionForPrompt(event, prompt)
			if err != nil || len(reply) == 0 {
				sendReaction(event, "❌")
			} else {
				sendMarkdown(event, strings.TrimPrefix(reply, "### Assistant:"))
			}

			Bot.client.UserTyping(event.RoomID, false, 0)

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

func sendReply(event *mevent.Event, text string) {
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
			Width:    cfg.Width,
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
		log.Error().Err(err).Msg("Failed to upload media")
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
