package main

import (
	"bot/store"

	"github.com/rs/zerolog"
	"maunium.net/go/mautrix"
	mcrypto "maunium.net/go/mautrix/crypto"
)

type BotType struct {
	client        *mautrix.Client
	configuration Configuration
	olmMachine    *mcrypto.OlmMachine
	stateStore    *store.StateStore
	txt2txt       *Txt2txt
	log           *zerolog.Logger
}
