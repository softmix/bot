package main

import (
	"bot/store"
	"context"
	"database/sql"
	"flag"
	"os"
	"os/signal"
	"syscall"

	// _ "github.com/motemen/go-loghttp/global"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go.mau.fi/util/dbutil"
	"maunium.net/go/mautrix"
	mcrypto "maunium.net/go/mautrix/crypto"
	mevent "maunium.net/go/mautrix/event"
	mid "maunium.net/go/mautrix/id"
)

var Bot BotType

func main() {
	configPath := flag.String("config", "config.yaml", "Path to the configuration file")
	dbFilename := flag.String("dbfile", "./state.db", "the SQLite DB file to use")
	flag.Parse()

	// Configure logging
	// log.SetFormatter(&log.JSONFormatter{})
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	log.Info().Msg("Starting")

	// Load configuration
	configBytes, err := os.ReadFile(*configPath)
	if err != nil {
		log.Fatal().Msgf("Couldn't open the configuration file at %s: %s", *configPath, err)
	}

	Bot.configuration = Configuration{}
	if err := Bot.configuration.Parse(configBytes); err != nil {
		log.Fatal().Msg("Failed to read config!")
	}

	username := mid.UserID(Bot.configuration.Username)
	_, _, err = username.Parse()
	if err != nil {
		log.Fatal().Msgf("Couldn't parse username: %s", username)
	}

	// Open the config database
	db, err := sql.Open("sqlite3", *dbFilename)
	if err != nil {
		log.Fatal().Msg("Could not open database.")
	}

	// Make sure to exit cleanly
	c := make(chan os.Signal, 1)
	signal.Notify(c,
		os.Interrupt,
		os.Kill,
		syscall.SIGABRT,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGQUIT,
		syscall.SIGTERM,
	)
	go func() {
		for range c { // when the process is killed
			log.Info().Msgf("'Cleaning up")
			db.Close()
			Bot.txt2txt.SaveHistories()
			os.Exit(0)
		}
	}()

	Bot.txt2txt = NewTxt2txt()
	err = Bot.txt2txt.LoadHistories()
	if err != nil {
		log.Fatal().Err(err).Msg("Couldn't load histories")
	}

	Bot.stateStore = store.NewStateStore(db)
	if err := Bot.stateStore.CreateTables(); err != nil {
		log.Fatal().Err(err).Msg("Failed to create tables.")
	}

	deviceID := FindDeviceID(db, username.String())
	if len(deviceID) > 0 {
		log.Info().Msgf("'Found existing device ID in database: %s", deviceID)
	}

	Bot.client, err = mautrix.NewClient(Bot.configuration.Homeserver, "", "")
	if err != nil {
		log.Fatal().Msg("Couldn't initialize the Matrix client")
	}

	_, err = DoRetry("login", func() (interface{}, error) {
		return Bot.client.Login(context.Background(), &mautrix.ReqLogin{
			Type: mautrix.AuthTypePassword,
			Identifier: mautrix.UserIdentifier{
				Type: mautrix.IdentifierTypeUser,
				User: username.String(),
			},
			Password:                 Bot.configuration.Password,
			InitialDeviceDisplayName: Bot.configuration.DisplayName,
			DeviceID:                 deviceID,
			StoreCredentials:         true,
		})
	})
	if err != nil {
		log.Fatal().Msgf("Couldn't login to the homeserver.")
	}
	log.Info().Msgf("Logged in as %s/%s", Bot.client.UserID, Bot.client.DeviceID)

	// set the client store on the client.
	Bot.client.Store = Bot.stateStore

	utilDb, err := dbutil.NewWithDB(db, "sqlite3")
	// Setup the crypto store
	sqlStore := mcrypto.NewSQLCryptoStore(
		utilDb,
		nil,
		username.String(),
		Bot.client.DeviceID,
		[]byte("standupbot_cryptostore_key"),
	)
	if err = sqlStore.DB.Upgrade(context.Background()); err != nil {
		log.Fatal().Msg("Could not upgrade tables for the SQL crypto store.")
	}

	Bot.olmMachine = mcrypto.NewOlmMachine(Bot.client, Bot.log, sqlStore, Bot.stateStore)
	err = Bot.olmMachine.Load(context.Background())
	if err != nil {
		log.Error().Msg("'Could not initialize encryption support. Encrypted rooms will not work.")
	}

	syncer := Bot.client.Syncer.(*mautrix.DefaultSyncer)
	// Hook up the OlmMachine into the Matrix client so it receives e2ee
	// keys and other such things.
	syncer.OnSync(func(_ context.Context, resp *mautrix.RespSync, since string) bool {
		Bot.olmMachine.ProcessSyncResponse(context.Background(), resp, since)
		return true
	})

	syncer.OnEventType(mevent.StateMember, func(ctx context.Context, event *mevent.Event) {
		Bot.olmMachine.HandleMemberEvent(ctx, event)
		Bot.stateStore.SetMembership(event)

		if event.GetStateKey() == username.String() && event.Content.AsMember().Membership == mevent.MembershipInvite {
			log.Info().Msgf("'Joining %s", event.RoomID)
			_, err := DoRetry("join room", func() (interface{}, error) {
				return Bot.client.JoinRoomByID(ctx, event.RoomID)
			})
			if err != nil {
				log.Error().Err(err).Msgf("'Could not join channel %s", event.RoomID.String())
			} else {
				log.Info().Msgf("'Joined %s sucessfully", event.RoomID.String())
			}
		} else if event.GetStateKey() == username.String() && event.Content.AsMember().Membership.IsLeaveOrBan() {
			log.Info().Msgf("'Left or banned from %s", event.RoomID)
		}
	})

	syncer.OnEventType(mevent.StateEncryption, func(_ context.Context, event *mevent.Event) {
		Bot.stateStore.SetEncryptionEvent(event)
	})

	syncer.OnEventType(mevent.EventMessage, func(ctx context.Context, event *mevent.Event) { go HandleMessage(ctx, event) })

	syncer.OnEventType(mevent.EventEncrypted, func(ctx context.Context, event *mevent.Event) {
		decryptedEvent, err := Bot.olmMachine.DecryptMegolmEvent(context.Background(), event)
		if err != nil {
			log.Error().Err(err).Msgf("'Failed to decrypt message from %s in %s", event.Sender, event.RoomID)
		} else {
			log.Debug().Msgf("'Received encrypted event from %s in %s", event.Sender, event.RoomID)
			if decryptedEvent.Type == mevent.EventMessage {
				go HandleMessage(ctx, decryptedEvent)
			}
		}
	})

	for {
		log.Debug().Msg("'Running sync...")
		err = Bot.client.Sync()
		if err != nil {
			log.Error().Err(err).Msg("Sync failed")
		}
	}
}

func FindDeviceID(db *sql.DB, accountID string) (deviceID mid.DeviceID) {
	err := db.QueryRow("SELECT device_id FROM crypto_account WHERE account_id=$1", accountID).Scan(&deviceID)
	if err != nil && err != sql.ErrNoRows {
		log.Warn().Err(err).Msg("Failed to scan device ID")
	}
	return
}
