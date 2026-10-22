package requests

import (
	"errors"
	"testing"
)

func TestStatusTransitions(t *testing.T) {
	// Explicit matrix of the agreed business workflow, including cancellation
	// after delivery and no-op retries of an already applied status.
	allowed := map[Status]map[Status]bool{
		Created:   {Created: true, Delivery: true, Cancelled: true},
		Delivery:  {Delivery: true, Delivered: true, Cancelled: true},
		Delivered: {Delivered: true, Cancelled: true},
		Cancelled: {Cancelled: true},
	}
	statuses := []Status{Created, Delivery, Delivered, Cancelled, "unknown"}
	for _, from := range statuses {
		for _, to := range statuses {
			t.Run(string(from)+"/"+string(to), func(t *testing.T) {
				change, err := ChangeStatus(from, to, "  Customer cancelled  ")
				if !allowed[from][to] {
					if !errors.Is(err, ErrInvalidTransition) {
						t.Fatalf("got %v, want invalid transition", err)
					}
					if change != (StatusChange{}) {
						t.Fatalf("invalid transition produced change: %+v", change)
					}
					return
				}
				if err != nil || change.From != from || change.To != to || change.Changed != (from != to) {
					t.Fatalf("change=%+v, err=%v", change, err)
				}
				wantComment := ""
				if to == Cancelled && from != to {
					wantComment = "Customer cancelled"
				}
				if change.CancellationComment != wantComment {
					t.Fatalf("unexpected history comment: %q", change.CancellationComment)
				}
			})
		}
	}
}

func TestCancellationAlwaysRequiresComment(t *testing.T) {
	for _, from := range []Status{Created, Delivery, Delivered, Cancelled} {
		for _, comment := range []string{"", " \t\n", "\u00a0\u2003"} {
			change, err := ChangeStatus(from, Cancelled, comment)
			if !errors.Is(err, ErrCancellationComment) || change != (StatusChange{}) {
				t.Fatalf("from=%s, comment=%q: change=%+v, err=%v", from, comment, change, err)
			}
		}
	}
}
