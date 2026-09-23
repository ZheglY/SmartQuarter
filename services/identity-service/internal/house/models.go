package house

import (
	"github.com/google/uuid"
	"strings"
	"unicode"
)

// Command is an internal use-case input. Public RPCs have dedicated typed requests.
type Command struct {
	PlatformAdminOverride            bool   `json:"platform_admin_override"`
	ID                               string `json:"id"`
	HouseID                          string `json:"house_id"`
	Name                             string `json:"name"`
	Address                          string `json:"address"`
	City                             string `json:"city"`
	Reason                           string `json:"reason"`
	Query                            string `json:"query"`
	Token                            string `json:"token"`
	TargetUserID                     string `json:"target_user_id"`
	UserID                           string `json:"user_id"`
	AfterUserID                      string `json:"after_user_id"`
	Category                         string `json:"category"`
	ExpiresInHours                   int    `json:"expires_in_hours"`
	MaxUses                          int    `json:"max_uses"`
	NotificationsEnabled             bool   `json:"notifications_enabled"`
	IssueNotificationsEnabled        bool   `json:"issue_notifications_enabled"`
	AnnouncementNotificationsEnabled bool   `json:"announcement_notifications_enabled"`
	MembershipNotificationsEnabled   bool   `json:"membership_notifications_enabled"`
	BotNotificationsEnabled          bool   `json:"bot_notifications_enabled"`
}

func NormalizeAddress(city, address string) string { return normalize(city) + "|" + normalize(address) }
func normalize(s string) string {
	s = strings.ReplaceAll(strings.ToLower(s), "ё", "е")
	return strings.Join(strings.FieldsFunc(s, func(r rune) bool { return unicode.IsSpace(r) || strings.ContainsRune(",.;", r) }), " ")
}
func validID(s string) bool {
	v, e := uuid.Parse(s)
	return e == nil && v != uuid.Nil && v.String() == s
}
func textOK(s string, max int) bool {
	return strings.TrimSpace(s) != "" && len([]rune(s)) <= max && !strings.ContainsAny(s, "<>\x00")
}
