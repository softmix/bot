//
// Implements the mautrix.Storer interface on StateStore
//

package store

import (
	"context"

	"github.com/rs/zerolog/log"
	"maunium.net/go/mautrix"
	mid "maunium.net/go/mautrix/id"
)

func (store *StateStore) SaveFilterID(_ context.Context, userID mid.UserID, filterID string) error {
	log.Debug().Msg("Upserting row into user_filter_ids")
	tx, err := store.DB.Begin()
	if err != nil {
		tx.Rollback()
		return err
	}

	update := "UPDATE user_filter_ids SET filter_id = ? WHERE user_id = ?"
	if _, err := tx.Exec(update, filterID, userID); err != nil {
		tx.Rollback()
		return err
	}

	insert := "INSERT OR IGNORE INTO user_filter_ids VALUES (?, ?)"
	if _, err := tx.Exec(insert, userID, filterID); err != nil {
		tx.Rollback()
		return err
	}

	tx.Commit()
	return nil
}

func (store *StateStore) LoadFilterID(_ context.Context, userID mid.UserID) (string, error) {
	row := store.DB.QueryRow("SELECT filter_id FROM user_filter_ids WHERE user_id = ?", userID)
	var filterID string
	if err := row.Scan(&filterID); err != nil {
		return "", err
	}
	return filterID, nil
}

func (store *StateStore) SaveNextBatch(_ context.Context, userID mid.UserID, nextBatchToken string) error {
	log.Debug().Msg("Upserting row into user_batch_tokens")
	tx, err := store.DB.Begin()
	if err != nil {
		tx.Rollback()
		return err
	}

	update := "UPDATE user_batch_tokens SET next_batch_token = ? WHERE user_id = ?"
	if _, err := tx.Exec(update, nextBatchToken, userID); err != nil {
		tx.Rollback()
		return err
	}

	insert := "INSERT OR IGNORE INTO user_batch_tokens VALUES (?, ?)"
	if _, err := tx.Exec(insert, userID, nextBatchToken); err != nil {
		tx.Rollback()
		return err
	}

	tx.Commit()
	return nil
}

func (store *StateStore) LoadNextBatch(_ context.Context, userID mid.UserID) (string, error) {
	row := store.DB.QueryRow("SELECT next_batch_token FROM user_batch_tokens WHERE user_id = ?", userID)
	var batchToken string
	if err := row.Scan(&batchToken); err != nil {
		return "", err
	}
	return batchToken, nil
}

func (store *StateStore) GetRoomMembers(roomId mid.RoomID) []mid.UserID {
	rows, err := store.DB.Query("SELECT user_id FROM room_members WHERE room_id = ?", roomId)
	users := make([]mid.UserID, 0)
	if err != nil {
		return users
	}
	defer rows.Close()

	var userId mid.UserID
	for rows.Next() {
		if err := rows.Scan(&userId); err == nil {
			users = append(users, userId)
		}
	}
	return users
}

func (store *StateStore) SaveRoom(room *mautrix.Room) {
	// This isn't really used at all.
}

func (store *StateStore) LoadRoom(roomId mid.RoomID) *mautrix.Room {
	// This isn't really used at all.
	return mautrix.NewRoom(roomId)
}
