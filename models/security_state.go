package models

import (
	"errors"
	"time"

	"kleingarten-verwaltung/securestore"
)

const maxKeyReveals = 3

type BackupKeyState struct {
	Fingerprint      string
	Source           string
	RevealCount      int
	RevealsRemaining int
	Acknowledged     bool
	LastRevealAt     *time.Time
}

func GetBackupKeyState() (*BackupKeyState, error) {
	if _, err := DB.Exec(`
	INSERT INTO app_security_state (id, key_reveal_count, key_acknowledged, updated_at)
	VALUES (1, 0, 0, CURRENT_TIMESTAMP)
	ON CONFLICT(id) DO NOTHING
	`); err != nil {
		return nil, err
	}

	row := DB.QueryRow(`SELECT key_reveal_count, key_acknowledged, key_last_reveal_at FROM app_security_state WHERE id = 1`)
	state := &BackupKeyState{}
	var lastReveal interface{}
	if err := row.Scan(&state.RevealCount, &state.Acknowledged, &lastReveal); err != nil {
		return nil, err
	}

	state.RevealsRemaining = maxKeyReveals - state.RevealCount
	if state.RevealsRemaining < 0 {
		state.RevealsRemaining = 0
	}

	if ts, ok := toTimePointer(lastReveal); ok {
		state.LastRevealAt = ts
	}

	fingerprint, err := securestore.KeyFingerprint()
	if err != nil {
		return nil, err
	}
	state.Fingerprint = fingerprint
	state.Source = securestore.KeySource()

	return state, nil
}

func RevealBackupKey() (string, *BackupKeyState, error) {
	state, err := GetBackupKeyState()
	if err != nil {
		return "", nil, err
	}

	if state.RevealCount >= maxKeyReveals {
		return "", state, errors.New("anzeigelimit erreicht")
	}

	if _, err := DB.Exec(`
	UPDATE app_security_state
	SET key_reveal_count = key_reveal_count + 1,
		key_last_reveal_at = CURRENT_TIMESTAMP,
		updated_at = CURRENT_TIMESTAMP
	WHERE id = 1
	`); err != nil {
		return "", nil, err
	}

	keyValue, err := securestore.KeyBase64()
	if err != nil {
		return "", nil, err
	}

	updatedState, err := GetBackupKeyState()
	if err != nil {
		return "", nil, err
	}

	return keyValue, updatedState, nil
}

func AcknowledgeBackupKeySaved() error {
	_, err := DB.Exec(`
	UPDATE app_security_state
	SET key_acknowledged = 1,
		updated_at = CURRENT_TIMESTAMP
	WHERE id = 1
	`)
	return err
}
