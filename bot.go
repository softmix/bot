package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/crypto"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

type txt2img_request struct {
	EnableHR          bool     `json:"enable_hr,omitempty"`
	DenoisingStrength float32  `json:"denoising_strength,omitempty"`
	FirstphaseWidth   int      `json:"firstphase_width,omitempty"`
	FirstphaseHeight  int      `json:"firstphase_height,omitempty"`
	Prompt            string   `json:"prompt,omitempty"`
	Styles            []string `json:"styles,omitempty"`
	Seed              int      `json:"seed,omitempty"`
	Subseed           int      `json:"subseed,omitempty"`
	SubseedStrength   float32  `json:"subseed_strength,omitempty"`
	SeedResizeFromH   int      `json:"seed_resize_from_h,omitempty"`
	SeedResizeFromW   int      `json:"seed_resize_from_w,omitempty"`
	SamplerName       string   `json:"sampler_name,omitempty"`
	BatchSize         int      `json:"batch_size,omitempty"`
	NIter             int      `json:"n_iter,omitempty"`
	Steps             int      `json:"steps,omitempty"`
	CfgScale          float32  `json:"cfg_scale,omitempty"`
	Width             int      `json:"width,omitempty"`
	Height            int      `json:"height,omitempty"`
	RestoreFaces      bool     `json:"restore_faces,omitempty"`
	Tiling            bool     `json:"tiling,omitempty"`
	NegativePrompt    string   `json:"negative_prompt,omitempty"`
	Eta               float32  `json:"eta,omitempty"`
	SChurn            float32  `json:"s_churn,omitempty"`
	STmax             float32  `json:"s_tmax,omitempty"`
	STmin             float32  `json:"s_tmin,omitempty"`
	SNoise            float32  `json:"s_noise,omitempty"`
	OverrideSettings  struct{} `json:"override_settings,omitempty"`
	SamplerIndex      string   `json:"sampler_index,omitempty"`
}

type txt2img_response struct {
	Images     []string `json:"images"`
	Parameters struct {
		EnableHr          bool        `json:"enable_hr"`
		DenoisingStrength int         `json:"denoising_strength"`
		FirstphaseWidth   int         `json:"firstphase_width"`
		FirstphaseHeight  int         `json:"firstphase_height"`
		Prompt            string      `json:"prompt"`
		Styles            interface{} `json:"styles"`
		Seed              int         `json:"seed"`
		Subseed           int         `json:"subseed"`
		SubseedStrength   int         `json:"subseed_strength"`
		SeedResizeFromH   int         `json:"seed_resize_from_h"`
		SeedResizeFromW   int         `json:"seed_resize_from_w"`
		SamplerName       interface{} `json:"sampler_name"`
		BatchSize         int         `json:"batch_size"`
		NIter             int         `json:"n_iter"`
		Steps             int         `json:"steps"`
		CfgScale          float64     `json:"cfg_scale"`
		Width             int         `json:"width"`
		Height            int         `json:"height"`
		RestoreFaces      bool        `json:"restore_faces"`
		Tiling            bool        `json:"tiling"`
		NegativePrompt    interface{} `json:"negative_prompt"`
		Eta               interface{} `json:"eta"`
		SChurn            float64     `json:"s_churn"`
		STmax             interface{} `json:"s_tmax"`
		STmin             float64     `json:"s_tmin"`
		SNoise            float64     `json:"s_noise"`
		OverrideSettings  struct {
		} `json:"override_settings"`
		SamplerIndex string `json:"sampler_index"`
	} `json:"parameters"`
	Info string `json:"info"`
}

type config struct {
	SDAPIURL string `yaml:"sd_api_url"`
	Matrix   struct {
		AccessToken string `yaml:"access_token"`
		Password    string `yaml:"password"`
		UserID      string `yaml:"user_id"`
		HSURL       string `yaml:"hs_url"`
		DisplayName string `yaml:"display_name"`
		DebugRoom   string `yaml:"debug_room"`
	}
}

// Simple crypto.StateStore implementation that says all rooms are encrypted.
type fakeStateStore struct{}

var _ crypto.StateStore = &fakeStateStore{}

func (fss *fakeStateStore) IsEncrypted(roomID id.RoomID) bool {
	return true
}

func (fss *fakeStateStore) GetEncryptionEvent(roomID id.RoomID) *event.EncryptionEventContent {
	return &event.EncryptionEventContent{
		Algorithm:              id.AlgorithmMegolmV1,
		RotationPeriodMillis:   7 * 24 * 60 * 60 * 1000,
		RotationPeriodMessages: 100,
	}
}

func (fss *fakeStateStore) FindSharedRooms(userID id.UserID) []id.RoomID {
	return []id.RoomID{}
}

// Simple crypto.Logger implementation that just prints to stdout.
type fakeLogger struct{}

var _ crypto.Logger = &fakeLogger{}

func (f fakeLogger) Error(message string, args ...interface{}) {
	fmt.Printf("[ERROR] "+message+"\n", args...)
}

func (f fakeLogger) Warn(message string, args ...interface{}) {
	fmt.Printf("[WARN] "+message+"\n", args...)
}

func (f fakeLogger) Debug(message string, args ...interface{}) {
	fmt.Printf("[DEBUG] "+message+"\n", args...)
}

func (f fakeLogger) Trace(message string, args ...interface{}) {
	if strings.HasPrefix(message, "Got membership state event") {
		return
	}
	fmt.Printf("[TRACE] "+message+"\n", args...)
}

// Easy way to get room members (to find out who to share keys to).
// In real apps, you should cache the member list somewhere and update it based on m.room.member events.
func getUserIDs(cli *mautrix.Client, roomID id.RoomID) []id.UserID {
	members, err := cli.JoinedMembers(roomID)
	if err != nil {
		panic(err)
	}
	userIDs := make([]id.UserID, len(members.Joined))
	i := 0
	for userID := range members.Joined {
		userIDs[i] = userID
		i++
	}
	return userIDs
}

func sendImage(mach *crypto.OlmMachine, cli *mautrix.Client, roomID id.RoomID, body string, url id.ContentURI) {
	content := event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    body,
		URL:     url.CUString(),
	}
	encrypted, err := mach.EncryptMegolmEvent(roomID, event.EventMessage, content)
	// These three errors mean we have to make a new Megolm session
	if err == crypto.SessionExpired || err == crypto.SessionNotShared || err == crypto.NoGroupSession {
		err = mach.ShareGroupSession(roomID, getUserIDs(cli, roomID))
		if err != nil {
			panic(err)
		}
		encrypted, err = mach.EncryptMegolmEvent(roomID, event.EventMessage, content)
	}
	if err != nil {
		panic(err)
	}

	resp, err := cli.SendMessageEvent(roomID, event.EventEncrypted, encrypted)
	if err != nil {
		panic(err)
	}
	fmt.Println("Send image response:", resp)
}

func sendMessage(mach *crypto.OlmMachine, cli *mautrix.Client, roomID id.RoomID, text string) {
	content := event.MessageEventContent{
		MsgType: "m.text",
		Body:    text,
	}
	encrypted, err := mach.EncryptMegolmEvent(roomID, event.EventMessage, content)
	// These three errors mean we have to make a new Megolm session
	if err == crypto.SessionExpired || err == crypto.SessionNotShared || err == crypto.NoGroupSession {
		err = mach.ShareGroupSession(roomID, getUserIDs(cli, roomID))
		if err != nil {
			panic(err)
		}
		encrypted, err = mach.EncryptMegolmEvent(roomID, event.EventMessage, content)
	}
	if err != nil {
		panic(err)
	}
	resp, err := cli.SendMessageEvent(roomID, event.EventEncrypted, encrypted)
	if err != nil {
		panic(err)
	}
	fmt.Println("Send response:", resp)
}

func main() {
	start := time.Now().UnixNano() / 1_000_000
	var cfg config

	configFile := flag.String("config", "config.yaml", "Path to the configuration file")

	flag.Parse()

	configBytes, err := ioutil.ReadFile(*configFile)
	if err != nil {
		panic(errors.Wrap(err, "Couldn't open the configuration file"))
	}

	if err := yaml.Unmarshal(configBytes, &cfg); err != nil {
		panic(errors.Wrap(err, "Couldn't read the configuration file"))
	}

	cli, err := mautrix.NewClient(cfg.Matrix.HSURL, "", "")
	if err != nil {
		panic(errors.Wrap(err, "Couldn't initialize the Matrix client"))
	}

	// Log in to get access token and device ID.
	_, err = cli.Login(&mautrix.ReqLogin{
		Type: mautrix.AuthTypePassword,
		Identifier: mautrix.UserIdentifier{
			Type: mautrix.IdentifierTypeUser,
			User: cfg.Matrix.UserID,
		},
		Password:                 cfg.Matrix.Password,
		InitialDeviceDisplayName: cfg.Matrix.DisplayName,
		StoreCredentials:         true,
	})
	if err != nil {
		panic(err)
	}

	// Log out when the program ends (don't do this in real apps)
	defer func() {
		fmt.Println("Logging out")
		resp, err := cli.Logout()
		if err != nil {
			fmt.Println("Logout error:", err)
		}
		fmt.Println("Logout response:", resp)
	}()

	// Create a store for the e2ee keys. In real apps, use NewSQLCryptoStore instead of NewGobStore.
	cryptoStore, err := crypto.NewGobStore("test.gob")
	if err != nil {
		panic(err)
	}

	mach := crypto.NewOlmMachine(cli, &fakeLogger{}, cryptoStore, &fakeStateStore{})
	// Load data from the crypto store
	err = mach.Load()
	if err != nil {
		panic(err)
	}

	// Hook up the OlmMachine into the Matrix client so it receives e2ee keys and other such things.
	syncer := cli.Syncer.(*mautrix.DefaultSyncer)
	syncer.OnSync(func(resp *mautrix.RespSync, since string) bool {
		mach.ProcessSyncResponse(resp, since)
		return true
	})
	syncer.OnEventType(event.StateMember, func(source mautrix.EventSource, evt *event.Event) {
		mach.HandleMemberEvent(evt)
	})
	// Listen to encrypted messages
	syncer.OnEventType(event.EventEncrypted, func(source mautrix.EventSource, evt *event.Event) {
		if evt.Timestamp < start {
			// Ignore events from before the program started
			return
		}
		decrypted, err := mach.DecryptMegolmEvent(evt)
		if err != nil {
			fmt.Println("Failed to decrypt:", err)
		} else {
			fmt.Println("Received encrypted event:", decrypted.Content.Raw)
			message, isMessage := decrypted.Content.Parsed.(*event.MessageEventContent)
			if isMessage && message.Body == "ping" {
				sendMessage(mach, cli, decrypted.RoomID, "Pong!")
			}

			if isMessage && message.Body == "yay" {
				cli.SendReaction(decrypted.RoomID, decrypted.ID, "🎉")
			}

			if isMessage && strings.HasPrefix(message.Body, "!gen ") {
				prompt := strings.TrimPrefix(message.Body, "!gen ")
				if len(prompt) == 0 {
					return
				}

				var req_body txt2img_request
				req_body.Prompt = prompt

				json_body, err := json.Marshal(req_body)
				if err != nil {
					panic(err)
				}

				cli.SendReaction(decrypted.RoomID, decrypted.ID, "👌")

				resp, err := http.Post(cfg.SDAPIURL, "application/json", bytes.NewBuffer(json_body))
				if err != nil {
					panic(err)
				}
				defer resp.Body.Close()

				fmt.Println("response Status:", resp.Status)
				fmt.Println("response Headers:", resp.Header)

				var res txt2img_response
				json.NewDecoder(resp.Body).Decode(&res)
				for _, encoded_image := range res.Images {
					image, _ := base64.StdEncoding.DecodeString(encoded_image)
					upload, err := cli.UploadMedia(mautrix.ReqUploadMedia{
						Content:       bytes.NewReader(image),
						ContentLength: int64(len(image)),
						ContentType:   "image/png",
					})
					if err != nil {
						panic(err)
					}

					sendImage(mach, cli, decrypted.RoomID, "image.png", upload.ContentURI)
				}

				cli.SendReaction(decrypted.RoomID, decrypted.ID, "✔️")
			}
		}
	})

	// Start long polling in the background
	go func() {
		err = cli.Sync()
		if err != nil {
			panic(err)
		}
	}()

	reader := bufio.NewReader(os.Stdin)
	for {
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		go sendMessage(mach, cli, id.RoomID(cfg.Matrix.DebugRoom), line)
	}
}
