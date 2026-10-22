package requests

import (
	"errors"
	"strings"
)

type Status string

const (
	Created   Status = "created"
	Delivery  Status = "delivery"
	Delivered Status = "delivered"
	Cancelled Status = "cancelled"
)

var (
	ErrInvalidTransition   = errors.New("invalid status transition")
	ErrCancellationComment = errors.New("cancellation comment is required")
)

// StatusChange describes a validated change, not a persisted history event.
// The persistence layer must check the expected version and save the status,
// comment, actor and timestamp atomically. Changed=false means no history write.
type StatusChange struct {
	From                Status
	To                  Status
	CancellationComment string
	Changed             bool
}

func ChangeStatus(from, to Status, cancellationComment string) (StatusChange, error) {
	if !validStatus(from) || !validStatus(to) {
		return StatusChange{}, ErrInvalidTransition
	}
	comment := strings.TrimSpace(cancellationComment)
	if to == Cancelled && comment == "" {
		return StatusChange{}, ErrCancellationComment
	}
	if from == to {
		return StatusChange{From: from, To: to}, nil
	}
	allowed := (from == Created && to == Delivery) ||
		(from == Delivery && to == Delivered) ||
		(from != Cancelled && to == Cancelled)
	if !allowed {
		return StatusChange{}, ErrInvalidTransition
	}
	change := StatusChange{From: from, To: to, Changed: true}
	if to == Cancelled {
		change.CancellationComment = comment
	}
	return change, nil
}

func validStatus(status Status) bool {
	switch status {
	case Created, Delivery, Delivered, Cancelled:
		return true
	default:
		return false
	}
}
