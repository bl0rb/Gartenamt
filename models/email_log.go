package models

import "time"

type EmailLog struct {
	ID              int       `json:"id"`
	ParzelleID      int       `json:"parzelle_id"`
	Recipient       string    `json:"recipient"`
	Subject         string    `json:"subject"`
	Message         string    `json:"message"`
	AttachmentTypes string    `json:"attachment_types"`
	Status          string    `json:"status"`
	ErrorMessage    string    `json:"error_message"`
	CreatedBy       string    `json:"created_by"`
	CreatedAt       time.Time `json:"created_at"`
}

func (l *EmailLog) Save() error {
	query := `INSERT INTO email_logs (parzelle_id, recipient, subject, message, attachment_types, status, error_message, created_by)
	          VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	result, err := DB.Exec(query, l.ParzelleID, l.Recipient, l.Subject, l.Message, l.AttachmentTypes, l.Status, l.ErrorMessage, l.CreatedBy)
	if err != nil {
		return err
	}

	id, _ := result.LastInsertId()
	l.ID = int(id)
	return nil
}

func GetEmailLogsByParzelleID(parzelleID int) ([]EmailLog, error) {
	query := `SELECT id, parzelle_id, recipient, subject, message, attachment_types, status, error_message, created_by, created_at
		FROM email_logs
		WHERE parzelle_id = ?
		ORDER BY created_at DESC`

	rows, err := DB.Query(query, parzelleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	logs := make([]EmailLog, 0)
	for rows.Next() {
		var logEntry EmailLog
		if err := rows.Scan(&logEntry.ID, &logEntry.ParzelleID, &logEntry.Recipient, &logEntry.Subject, &logEntry.Message, &logEntry.AttachmentTypes, &logEntry.Status, &logEntry.ErrorMessage, &logEntry.CreatedBy, &logEntry.CreatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, logEntry)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return logs, nil
}
