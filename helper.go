package main

import (
	"context"
	"errors"
	"fmt"
	_ "strconv"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/sethvargo/go-retry"
	"maunium.net/go/mautrix"
	mcrypto "maunium.net/go/mautrix/crypto"
	mevent "maunium.net/go/mautrix/event"
	mid "maunium.net/go/mautrix/id"
)

func DoRetry(description string, fn func() (interface{}, error)) (interface{}, error) {
	var err error
	b := retry.NewFibonacci(1 * time.Second)
	if err != nil {
		panic(err)
	}
	b = retry.WithMaxRetries(5, b)
	for {
		log.Info().Msgf("trying: %s", description)
		var val interface{}
		val, err = fn()
		if err == nil {
			log.Info().Msgf("%s succeeded", description)
			return val, nil
		}
		nextDuration, stop := b.Next()
		log.Debug().Msgf("  %s failed. Retrying in %f seconds...", description, nextDuration.Seconds())
		if stop {
			log.Debug().Msgf("  %s failed. Retry limit reached. Will not retry.", description)
			err = errors.New("%s failed. Retry limit reached. Will not retry.")
			break
		}
		time.Sleep(nextDuration)
	}
	return nil, err
}

func SendMessage(roomId mid.RoomID, content *mevent.MessageEventContent) (resp *mautrix.RespSendEvent, err error) {
	eventContent := &mevent.Content{Parsed: content}
	r, err := DoRetry(fmt.Sprintf("send message to %s", roomId), func() (interface{}, error) {
		isEncrypted, err := Bot.stateStore.IsEncrypted(context.Background(), roomId)
		if err != nil {
			log.Error().Err(err).Msg("Error checking if the state store is encrypted")
			return nil, err
		}
		if isEncrypted {
			log.Debug().Msgf("Sending encrypted event to %s", roomId)
			encrypted, err := Bot.olmMachine.EncryptMegolmEvent(context.Background(), roomId, mevent.EventMessage, eventContent)

			// These three errors mean we have to make a new Megolm session
			if err == mcrypto.SessionExpired || err == mcrypto.SessionNotShared || err == mcrypto.NoGroupSession {
				err = Bot.olmMachine.ShareGroupSession(context.Background(), roomId, Bot.stateStore.GetRoomMembers(roomId))
				if err != nil {
					log.Error().Err(err).Msgf("Failed to share group session to %s", roomId)
					return nil, err
				}

				encrypted, err = Bot.olmMachine.EncryptMegolmEvent(context.Background(), roomId, mevent.EventMessage, eventContent)
			}

			if err != nil {
				log.Error().Err(err).Msgf("Failed to encrypt message to %s", roomId)
				return nil, err
			}

			encrypted.RelatesTo = content.RelatesTo // The m.relates_to field should be unencrypted, so copy it.
			return Bot.client.SendMessageEvent(context.Background(), roomId, mevent.EventEncrypted, encrypted)
		} else {
			log.Debug().Msgf("Sending unencrypted event to %s", roomId)
			return Bot.client.SendMessageEvent(context.Background(), roomId, mevent.EventMessage, eventContent)
		}
	})
	if err != nil {
		// give up
		log.Error().Err(err).Msgf("Failed to send message to %s", roomId)
		return nil, err
	}
	return r.(*mautrix.RespSendEvent), err
}
