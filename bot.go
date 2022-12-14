package main

import (
	"bot/store"
	"database/sql"
	"flag"
	"os"
	"os/signal"
	"syscall"

	log "github.com/sirupsen/logrus"
	"maunium.net/go/mautrix"
	mcrypto "maunium.net/go/mautrix/crypto"
	mevent "maunium.net/go/mautrix/event"
	mid "maunium.net/go/mautrix/id"
	"maunium.net/go/mautrix/util/dbutil"
)

var Bot BotType

func main() {
	configPath := flag.String("config", "config.yaml", "Path to the configuration file")
	dbFilename := flag.String("dbfile", "./state.db", "the SQLite DB file to use")
	flag.Parse()

	// Configure logging
	log.SetFormatter(&log.JSONFormatter{})
	log.SetLevel(log.DebugLevel)
	log.Info("Starting")

	// Load configuration
	configBytes, err := os.ReadFile(*configPath)
	if err != nil {
		log.Fatalf("Couldn't open the configuration file at %s: %s", *configPath, err)
	}

	Bot.configuration = Configuration{}
	if err := Bot.configuration.Parse(configBytes); err != nil {
		log.Fatal("Failed to read config!")
	}
	username := mid.UserID(Bot.configuration.Username)
	_, _, err = username.Parse()
	if err != nil {
		log.Fatalf("Couldn't parse username: %s", username)
	}

	// Open the config database
	db, err := sql.Open("sqlite3", *dbFilename)
	if err != nil {
		log.Fatal("Could not open database.")
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
			log.Info("Cleaning up")
			db.Close()
			os.Exit(0)
		}
	}()

	Bot.stateStore = store.NewStateStore(db)
	if err := Bot.stateStore.CreateTables(); err != nil {
		log.Fatal("Failed to create tables.", err)
	}

	deviceID := FindDeviceID(db, username.String())
	if len(deviceID) > 0 {
		log.Info("Found existing device ID in database:", deviceID)
	}

	Bot.client, err = mautrix.NewClient(Bot.configuration.Homeserver, "", "")
	if err != nil {
		log.Fatal("Couldn't initialize the Matrix client")
	}

	_, err = DoRetry("login", func() (interface{}, error) {
		return Bot.client.Login(&mautrix.ReqLogin{
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
		log.Fatalf("Couldn't login to the homeserver.")
	}
	log.Infof("Logged in as %s/%s", Bot.client.UserID, Bot.client.DeviceID)

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
	if err = sqlStore.DB.Upgrade(); err != nil {
		log.Fatal("Could not upgrade tables for the SQL crypto store.")
	}

	Bot.olmMachine = mcrypto.NewOlmMachine(Bot.client, &CryptoLogger{}, sqlStore, Bot.stateStore)
	err = Bot.olmMachine.Load()
	if err != nil {
		log.Errorf("Could not initialize encryption support. Encrypted rooms will not work.")
	}

	syncer := Bot.client.Syncer.(*mautrix.DefaultSyncer)
	// Hook up the OlmMachine into the Matrix client so it receives e2ee
	// keys and other such things.
	syncer.OnSync(func(resp *mautrix.RespSync, since string) bool {
		Bot.olmMachine.ProcessSyncResponse(resp, since)
		return true
	})

	syncer.OnEventType(mevent.StateMember, func(_ mautrix.EventSource, event *mevent.Event) {
		Bot.olmMachine.HandleMemberEvent(event)
		Bot.stateStore.SetMembership(event)

		if event.GetStateKey() == username.String() && event.Content.AsMember().Membership == mevent.MembershipInvite {
			log.Info("Joining ", event.RoomID)
			_, err := DoRetry("join room", func() (interface{}, error) {
				return Bot.client.JoinRoomByID(event.RoomID)
			})
			if err != nil {
				log.Errorf("Could not join channel %s. Error %+v", event.RoomID.String(), err)
			} else {
				log.Infof("Joined %s sucessfully", event.RoomID.String())
			}
		} else if event.GetStateKey() == username.String() && event.Content.AsMember().Membership.IsLeaveOrBan() {
			log.Infof("Left or banned from %s", event.RoomID)
		}
	})

	syncer.OnEventType(mevent.StateEncryption, func(_ mautrix.EventSource, event *mevent.Event) {
		Bot.stateStore.SetEncryptionEvent(event)
	})

	syncer.OnEventType(mevent.EventMessage, func(source mautrix.EventSource, event *mevent.Event) { go HandleMessage(source, event) })

	syncer.OnEventType(mevent.EventEncrypted, func(source mautrix.EventSource, event *mevent.Event) {
		decryptedEvent, err := Bot.olmMachine.DecryptMegolmEvent(event)
		if err != nil {
			log.Errorf("Failed to decrypt message from %s in %s: %+v", event.Sender, event.RoomID, err)
		} else {
			log.Debugf("Received encrypted event from %s in %s", event.Sender, event.RoomID)
			if decryptedEvent.Type == mevent.EventMessage {
				go HandleMessage(source, decryptedEvent)
			}
		}
	})

	for {
		log.Debugf("Running sync...")
		err = Bot.client.Sync()
		if err != nil {
			log.Errorf("Sync failed. %+v", err)
		}
	}
}

func FindDeviceID(db *sql.DB, accountID string) (deviceID mid.DeviceID) {
	err := db.QueryRow("SELECT device_id FROM crypto_account WHERE account_id=$1", accountID).Scan(&deviceID)
	if err != nil && err != sql.ErrNoRows {
		log.Warnf("Failed to scan device ID: %v", err)
	}
	return
}
