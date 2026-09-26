package house

import (
	"context"
	"github.com/jackc/pgx/v5"
	"strings"
)

const platformUserSQL = `SELECT jsonb_build_object('id',u.id,'display_name',u.display_name,'max_user_id',u.max_user_id::text,'can_register_house',EXISTS(SELECT 1 FROM chairman_permissions p WHERE p.user_id=u.id),'managed_houses',(SELECT count(*) FROM memberships m WHERE m.user_id=u.id AND m.role='CHAIRMAN' AND m.status='ACTIVE')) FROM users u `
const adminHouseSQL = `SELECT jsonb_build_object('id',h.id,'name',h.name,'address',h.address,'city',h.city,'chairman_user_id',COALESCE(m.user_id::text,''),'chairman_display_name',COALESCE(u.display_name,'')) FROM houses h LEFT JOIN memberships m ON m.house_id=h.id AND m.role='CHAIRMAN' AND m.status='ACTIVE' LEFT JOIN users u ON u.id=m.user_id `

func chairmanAllowed(ctx context.Context, tx pgx.Tx, user string) (bool, error) {
	var yes bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM chairman_permissions WHERE user_id=$1)`, user).Scan(&yes)
	return yes, err
}

func (s *Service) administration(ctx context.Context, tx pgx.Tx, a actor, op string, c Command) (record, error) {
	if !a.Admin {
		return nil, denied()
	}
	if len([]rune(c.Query)) > 255 {
		return nil, invalid()
	}
	switch op {
	case "ListPlatformUsers":
		rs, e := rows(ctx, tx, platformUserSQL+`WHERE $1='' OR position(lower($1) in lower(u.display_name||' '||u.id::text||' '||u.max_user_id::text))>0 ORDER BY u.created_at DESC,u.id LIMIT 100`, strings.TrimSpace(c.Query))
		return list(rs, ""), e
	case "ListAdminHouses":
		rs, e := rows(ctx, tx, adminHouseSQL+`WHERE $1='' OR position(lower($1) in lower(h.city||' '||h.address||' '||h.name))>0 ORDER BY h.city,h.address,h.id LIMIT 100`, strings.TrimSpace(c.Query))
		return list(rs, ""), e
	case "GrantChairmanPermission", "RevokeChairmanPermission":
		if !validID(c.UserID) {
			return nil, invalid()
		}
		if _, e := one(ctx, tx, platformUserSQL+`WHERE u.id=$1`, c.UserID); e != nil {
			return nil, e
		}
		if op == "GrantChairmanPermission" {
			if _, e := tx.Exec(ctx, `INSERT INTO chairman_permissions(user_id,granted_by) VALUES($1,$2) ON CONFLICT DO NOTHING`, c.UserID, a.User); e != nil {
				return nil, e
			}
		} else {
			if _, e := tx.Exec(ctx, `DELETE FROM chairman_permissions WHERE user_id=$1`, c.UserID); e != nil {
				return nil, e
			}
			if _, e := tx.Exec(ctx, `UPDATE memberships SET role='RESIDENT',updated_at=now() WHERE user_id=$1 AND role='CHAIRMAN'`, c.UserID); e != nil {
				return nil, e
			}
			if _, e := tx.Exec(ctx, `UPDATE house_registrations SET status='CANCELLED',reviewed_at=now(),reviewed_by=$2 WHERE applicant_user_id=$1 AND status='PENDING'`, c.UserID, a.User); e != nil {
				return nil, e
			}
			if _, e := tx.Exec(ctx, `UPDATE chairman_transfers SET status='CANCELLED' WHERE (current_chairman_user_id=$1 OR target_user_id=$1) AND status='PENDING'`, c.UserID); e != nil {
				return nil, e
			}
		}
		if e := audit(ctx, tx, a, op, c.UserID, ""); e != nil {
			return nil, e
		}
		return one(ctx, tx, platformUserSQL+`WHERE u.id=$1`, c.UserID)
	case "AssignHouseChairman", "RemoveHouseChairman":
		if !validID(c.HouseID) {
			return nil, invalid()
		}
		if _, e := one(ctx, tx, adminHouseSQL+`WHERE h.id=$1`, c.HouseID); e != nil {
			return nil, e
		}
		if op == "AssignHouseChairman" && !validID(c.TargetUserID) {
			return nil, invalid()
		}
		if _, e := tx.Exec(ctx, `UPDATE memberships SET role='RESIDENT',updated_at=now() WHERE house_id=$1 AND role='CHAIRMAN'`, c.HouseID); e != nil {
			return nil, e
		}
		if _, e := tx.Exec(ctx, `UPDATE chairman_transfers SET status='CANCELLED' WHERE house_id=$1 AND status='PENDING'`, c.HouseID); e != nil {
			return nil, e
		}
		if op == "AssignHouseChairman" {
			if _, e := tx.Exec(ctx, `INSERT INTO chairman_permissions(user_id,granted_by) VALUES($1,$2) ON CONFLICT DO NOTHING`, c.TargetUserID, a.User); e != nil {
				return nil, e
			}
			if _, e := tx.Exec(ctx, `INSERT INTO memberships(user_id,house_id,role,status) VALUES($1,$2,'CHAIRMAN','ACTIVE') ON CONFLICT(user_id,house_id) DO UPDATE SET role='CHAIRMAN',status='ACTIVE',updated_at=now()`, c.TargetUserID, c.HouseID); e != nil {
				return nil, e
			}
			if _, e := tx.Exec(ctx, `UPDATE users SET default_house_id=COALESCE(default_house_id,$2::uuid),updated_at=now() WHERE id=$1`, c.TargetUserID, c.HouseID); e != nil {
				return nil, e
			}
		}
		if e := audit(ctx, tx, a, op, c.HouseID, c.HouseID); e != nil {
			return nil, e
		}
		return one(ctx, tx, adminHouseSQL+`WHERE h.id=$1`, c.HouseID)
	}
	return nil, invalid()
}
