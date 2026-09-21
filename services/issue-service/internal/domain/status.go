package domain

type Status string

const (
	Detected         Status = "DETECTED"
	Confirming       Status = "CONFIRMING"
	ReadyForAppeal   Status = "READY_FOR_APPEAL"
	HandedToChairman Status = "HANDED_TO_CHAIRMAN"
	MarkedSent       Status = "MARKED_SENT"
	WaitingResult    Status = "WAITING_RESULT"
	Resolved         Status = "RESOLVED"
)

func (s Status) Valid() bool {
	switch s {
	case Detected, Confirming, ReadyForAppeal, HandedToChairman, MarkedSent, WaitingResult, Resolved:
		return true
	}
	return false
}
func Transition(current, target Status, role Role) error {
	if role != Chairman && role != Admin {
		return ErrPermission
	}
	if !current.Valid() || !target.Valid() {
		return Fail(ErrInvalid, "invalid status")
	}
	next := map[Status]Status{Detected: Confirming, Confirming: ReadyForAppeal, ReadyForAppeal: HandedToChairman, HandedToChairman: MarkedSent, MarkedSent: WaitingResult, WaitingResult: Resolved}
	if next[current] == target {
		return nil
	}
	if (current == Detected || current == Confirming) && target == Resolved {
		return nil
	}
	// The only extra administrative correction: reopen a resolved issue for confirmation.
	if role == Admin && current == Resolved && target == Confirming {
		return nil
	}
	return Fail(ErrPrecondition, "status transition not allowed")
}
