package house

import (
	"context"
	"github.com/jackc/pgx/v5"
)

func (s *Service) preferences(ctx context.Context, tx pgx.Tx, a actor, op string, c Command) (record, error) {
	switch op {
	case "GetHouseAccessState":
		return one(ctx, tx, `SELECT jsonb_build_object('platform_admin',$2::boolean,'can_register_house',($2 OR EXISTS(SELECT 1 FROM chairman_permissions WHERE user_id=$1)),'can_manage_active_house',EXISTS(SELECT 1 FROM houses h WHERE h.id=NULLIF($3,'')::uuid AND ($2 OR EXISTS(SELECT 1 FROM memberships m WHERE m.house_id=h.id AND m.user_id=$1 AND m.status='ACTIVE' AND m.role IN ('CHAIRMAN','ADMIN')))),'pending_registrations',(SELECT count(*) FROM house_registrations WHERE applicant_user_id=$1 AND status='PENDING'),'pending_join_requests',(SELECT count(*) FROM join_requests WHERE user_id=$1 AND status='PENDING'),'incoming_join_requests',(SELECT count(*) FROM join_requests j JOIN memberships m ON m.house_id=j.house_id WHERE m.user_id=$1 AND m.status='ACTIVE' AND m.role IN ('CHAIRMAN','ADMIN') AND j.status='PENDING' AND j.house_id=NULLIF($3,'')::uuid))`, a.User, a.Admin, a.House)
	case "GetNotificationPreferences", "UpdateNotificationPreferences":
		if _, e := tx.Exec(ctx, `INSERT INTO notification_preferences(user_id) VALUES($1) ON CONFLICT DO NOTHING`, a.User); e != nil {
			return nil, e
		}
		if op == "UpdateNotificationPreferences" {
			if _, e := tx.Exec(ctx, `UPDATE notification_preferences SET notifications_enabled=$2,issue_notifications_enabled=$3,announcement_notifications_enabled=$4,membership_notifications_enabled=$5,bot_notifications_enabled=$6 WHERE user_id=$1`, a.User, c.NotificationsEnabled, c.IssueNotificationsEnabled, c.AnnouncementNotificationsEnabled, c.MembershipNotificationsEnabled, c.BotNotificationsEnabled); e != nil {
				return nil, e
			}
			if e := audit(ctx, tx, a, "notifications.preferences.updated", a.User, ""); e != nil {
				return nil, e
			}
		}
		return one(ctx, tx, `SELECT to_jsonb(p) FROM notification_preferences p WHERE user_id=$1`, a.User)
	case "ListNotificationRecipients":
		if c.Category != "membership" && c.Category != "issue" && c.Category != "announcement" && c.Category != "join_request" {
			return nil, invalid()
		}
		if c.UserID == "" && !validID(c.HouseID) {
			return nil, invalid()
		}
		// Direct membership decisions can reach an applicant without an active
		// membership. Broadcasts and issue updates always require current access.
		rs, e := rows(ctx, tx, `SELECT jsonb_build_object('user_id',u.id,'max_user_id',u.max_user_id::text) FROM users u LEFT JOIN notification_preferences p ON p.user_id=u.id WHERE ($1='' OR u.id=NULLIF($1,'')::uuid) AND ($2='' OR u.id>NULLIF($2,'')::uuid) AND (($3='membership' AND $1<>'') OR EXISTS(SELECT 1 FROM memberships m WHERE m.user_id=u.id AND m.house_id=NULLIF($4,'')::uuid AND m.status='ACTIVE' AND ($3<>'join_request' OR m.role IN ('CHAIRMAN','ADMIN')))) AND COALESCE(p.notifications_enabled,true) AND COALESCE(p.bot_notifications_enabled,true) AND CASE $3 WHEN 'issue' THEN COALESCE(p.issue_notifications_enabled,true) WHEN 'announcement' THEN COALESCE(p.announcement_notifications_enabled,true) ELSE COALESCE(p.membership_notifications_enabled,true) END ORDER BY u.id LIMIT 100`, c.UserID, c.AfterUserID, c.Category, c.HouseID)
		return list(rs, ""), e
	}
	return nil, invalid()
}
