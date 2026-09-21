package domain

import (
	"errors"
	"testing"
)

func TestStatusTransitions(t *testing.T) {
	statuses := []Status{Detected, Confirming, ReadyForAppeal, HandedToChairman, MarkedSent, WaitingResult, Resolved}
	for i, from := range statuses {
		for j, to := range statuses {
			want := j == i+1 || (i <= 1 && to == Resolved)
			err := Transition(from, to, Chairman)
			if (err == nil) != want {
				t.Errorf("%s -> %s: %v", from, to, err)
			}
			if !errors.Is(Transition(from, to, Resident), ErrPermission) {
				t.Error("resident can change status")
			}
		}
	}
	if Transition(Resolved, Confirming, Admin) != nil {
		t.Error("admin cannot reopen")
	}
	if !errors.Is(Transition(Detected, "BAD", Admin), ErrInvalid) {
		t.Error("invalid enum accepted")
	}
}
func TestAttachmentTransitions(t *testing.T) {
	for _, from := range []AttachmentStatus{Uploading, Ready, Attached, Rejected, Expired} {
		for _, to := range []AttachmentStatus{Uploading, Ready, Attached, Rejected, Expired} {
			a := Attachment{Status: from}
			want := from == Uploading && (to == Ready || to == Rejected || to == Expired) || from == Ready && to == Attached
			if (a.Transition(to) == nil) != want {
				t.Errorf("%s -> %s", from, to)
			}
		}
	}
}
