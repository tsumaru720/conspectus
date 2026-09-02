package domain

import (
	"errors"
	"fmt"
	"time"
)

var (
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
	ErrValidation   = errors.New("validation error")
	ErrReadOnly     = errors.New("read only")
	ErrUnauthorized = errors.New("unauthorized")
)

type ConflictError struct {
	Message    string
	ExistingID int32
}

func (e *ConflictError) Error() string { return e.Message }

func (e *ConflictError) Unwrap() error { return ErrConflict }

type ValidationError struct {
	Details []string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error: %v", e.Details)
}

func (e *ValidationError) Unwrap() error { return ErrValidation }

type Class struct {
	ID          int32  `json:"id"`
	Description string `json:"description"`
}

type Asset struct {
	ID          int32  `json:"id"`
	ClassID     int32  `json:"class_id"`
	Description string `json:"description"`
	Closed      bool   `json:"closed"`
}

type LogEntry struct {
	ID      int32     `json:"id"`
	AssetID int32     `json:"asset_id"`
	At      time.Time `json:"at"`
	Deposit Money     `json:"deposit"`
	Value   Money     `json:"value"`
}

type Payment struct {
	ID      int32     `json:"id"`
	AssetID int32     `json:"asset_id"`
	At      time.Time `json:"at"`
	Amount  Money     `json:"amount"`
}
