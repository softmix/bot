package main

import (
	"encoding/json"

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

	eventJson, _ := json.Marshal(event)
	log.Infof(string(eventJson))

	// content := format.RenderMarkdown(App.configuration.VacationMessage, true, true)
	// SendMessage(event.RoomID, &content)
}

// 		if isMessage && message.Body == "ping" {
// 			sendMessage(mach, cli, decrypted.RoomID, "Pong!")
// 		}

// 		if isMessage && message.Body == "yay" {
// 			cli.SendReaction(decrypted.RoomID, decrypted.ID, "🎉")
// 		}

// 		if isMessage && strings.HasPrefix(message.Body, "!gen ") {
// 			prompt := strings.TrimPrefix(message.Body, "!gen ")
// 			if len(prompt) == 0 {
// 				return
// 			}

// 			var req_body txt2img_request
// 			req_body.Prompt = prompt

// 			json_body, err := json.Marshal(req_body)
// 			if err != nil {
// 				log.Errorf("Failed to marshal fields to JSON, %w", err)
// 				return
// 			}

// 			cli.SendReaction(decrypted.RoomID, decrypted.ID, "👌")

// 			resp, err := http.Post(cfg.SDAPIURL, "application/json", bytes.NewBuffer(json_body))
// 			if err != nil {
// 				log.Errorf("Failed to POST to SD API, %w", err)
// 				return
// 			}
// 			defer resp.Body.Close()

// 			fmt.Println("response Status:", resp.Status)
// 			fmt.Println("response Headers:", resp.Header)

// 			var res txt2img_response
// 			if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
// 				log.Errorf("Couldn't decode the response, %w", err)
// 				return
// 			}
// 			for _, encoded_image := range res.Images {
// 				image, _ := base64.StdEncoding.DecodeString(encoded_image)
// 				upload, err := cli.UploadMedia(mautrix.ReqUploadMedia{
// 					Content:       bytes.NewReader(image),
// 					ContentLength: int64(len(image)),
// 					ContentType:   "image/png",
// 				})
// 				if err != nil {
// 					log.Errorf("Got an error when uploading media.")
// 					return
// 				}

// 				sendImage(mach, cli, decrypted.RoomID, "image.png", upload.ContentURI)
// 			}

// 			cli.SendReaction(decrypted.RoomID, decrypted.ID, "✔️")
// 		}
// 	}
// })
