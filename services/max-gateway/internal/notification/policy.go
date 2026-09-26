package notification

func quietEvent(kind string) bool {
	switch kind {
	case "poll.voted", "issue.created", "issue.confirmed",
		"service_contact.create", "service_contact.update", "service_contact.archive",
		"house.registration.created", "house.registration.cancelled", "house.created",
		"house.join.cancelled", "house.membership.created",
		"house.invite.created", "house.invite.redeemed", "house.invite.revoked":
		return true
	}
	return false
}
