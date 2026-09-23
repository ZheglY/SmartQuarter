package house

import (
	"context"
	"github.com/jackc/pgx/v5"
)

func member(ctx context.Context, tx pgx.Tx, id string) (record, error) {
	return one(ctx, tx, `SELECT to_jsonb(m)||jsonb_build_object('display_name',u.display_name) FROM memberships m JOIN users u ON u.id=m.user_id WHERE m.id=$1 FOR UPDATE OF m`, id)
}
func (s *Service) members(ctx context.Context, tx pgx.Tx, a actor, op string, c Command) (record, error) {
	if e := manager(ctx, tx, a, a.House); e != nil {
		return nil, e
	}
	if op == "ListHouseMembers" {
		rs, e := rows(ctx, tx, `SELECT to_jsonb(m)||jsonb_build_object('display_name',u.display_name) FROM memberships m JOIN users u ON u.id=m.user_id WHERE m.house_id=$1 ORDER BY u.display_name,m.id LIMIT 500`, a.House)
		return list(rs, ""), e
	}
	if !validID(c.ID) {
		return nil, invalid()
	}
	r, e := member(ctx, tx, c.ID)
	if e != nil {
		return nil, e
	}
	if r.str("house_id") != a.House {
		return nil, denied()
	}
	if r.str("role") == "CHAIRMAN" && (!a.Admin || !c.PlatformAdminOverride) {
		return nil, denied()
	}
	// A membership administrator cannot remove another administrator or chairman.
	if r.str("role") == "ADMIN" && (!a.Admin || !c.PlatformAdminOverride) {
		return nil, denied()
	}
	ev := "house.membership.deactivated"
	if op == "RemoveMembership" {
		_, e = tx.Exec(ctx, `DELETE FROM memberships WHERE id=$1`, c.ID)
		r["status"] = "REMOVED"
		ev = "house.membership.removed"
	} else {
		target := "INACTIVE"
		if op == "ReactivateMembership" {
			target = "ACTIVE"
			ev = "house.membership.activated"
		}
		if r.str("status") == target {
			return r, nil
		}
		_, e = tx.Exec(ctx, `UPDATE memberships SET status=$2,updated_at=now() WHERE id=$1`, c.ID, target)
		r["status"] = target
	}
	if e != nil {
		return nil, e
	}
	if r.str("status") != "ACTIVE" {
		if _, e = tx.Exec(ctx, `UPDATE users SET default_house_id=NULL,updated_at=now() WHERE id=$1 AND default_house_id=$2`, r.str("user_id"), a.House); e != nil {
			return nil, e
		}
	}
	e = emit(ctx, tx, a, ev, a.House, r.str("user_id"), c.ID)
	return r, e
}

func (s *Service) transfer(ctx context.Context, tx pgx.Tx, a actor, op string, c Command) (record, error) {
	const kind = "CHAIRMAN_TRANSFER_STATUS"
	// Expire pending offers before creating another one. Expiry is durable even
	// when an unrelated operation later fails only on the next successful call.
	if _, e := tx.Exec(ctx, `UPDATE chairman_transfers SET status='EXPIRED' WHERE status='PENDING' AND expires_at<=now()`); e != nil {
		return nil, e
	}
	if op == "ListChairmanTransfers" {
		rs, e := rows(ctx, tx, `SELECT to_jsonb(t) FROM chairman_transfers t WHERE current_chairman_user_id=$1 OR target_user_id=$1 ORDER BY created_at DESC LIMIT 100`, a.User)
		return list(rs, kind), e
	}
	if op == "CreateChairmanTransfer" {
		if !validID(c.TargetUserID) || c.TargetUserID == a.User {
			return nil, invalid()
		}
		if e := manager(ctx, tx, a, a.House); e != nil {
			return nil, e
		}
		var chairman string
		if e := tx.QueryRow(ctx, `SELECT user_id::text FROM memberships WHERE house_id=$1 AND role='CHAIRMAN' AND status='ACTIVE' FOR UPDATE`, a.House).Scan(&chairman); e != nil {
			return nil, e
		}
		if chairman != a.User {
			return nil, denied()
		}
		var resident bool
		if e := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM memberships WHERE house_id=$1 AND user_id=$2 AND role='RESIDENT' AND status='ACTIVE')`, a.House, c.TargetUserID).Scan(&resident); e != nil {
			return nil, e
		}
		if !resident {
			return nil, conflict()
		}
		r, e := one(ctx, tx, `INSERT INTO chairman_transfers(house_id,current_chairman_user_id,target_user_id) VALUES($1,$2,$3) RETURNING to_jsonb(chairman_transfers)`, a.House, a.User, c.TargetUserID)
		if e != nil {
			return nil, e
		}
		e = emit(ctx, tx, a, "house.chairman.transfer_requested", a.House, c.TargetUserID, r.str("id"))
		return decorate(r, kind), e
	}
	if !validID(c.ID) {
		return nil, invalid()
	}
	r, e := one(ctx, tx, `SELECT to_jsonb(t) FROM chairman_transfers t WHERE id=$1 FOR UPDATE`, c.ID)
	if e != nil {
		return nil, e
	}
	target := "ACCEPTED"
	if op == "RejectChairmanTransfer" {
		target = "REJECTED"
	}
	if op == "CancelChairmanTransfer" {
		target = "CANCELLED"
		if a.User != r.str("current_chairman_user_id") && !a.Admin {
			return nil, denied()
		}
	} else if a.User != r.str("target_user_id") {
		return nil, denied()
	}
	if e = transition(r.str("status"), target); e != nil {
		return nil, e
	}
	if r.str("status") == target {
		return decorate(r, kind), nil
	}
	if target == "ACCEPTED" {
		tag, e := tx.Exec(ctx, `UPDATE memberships SET role='RESIDENT',updated_at=now() WHERE house_id=$1 AND user_id=$2 AND role='CHAIRMAN' AND status='ACTIVE'`, r.str("house_id"), r.str("current_chairman_user_id"))
		if e != nil {
			return nil, e
		}
		if tag.RowsAffected() != 1 {
			return nil, conflict()
		}
		tag, e = tx.Exec(ctx, `UPDATE memberships SET role='CHAIRMAN',updated_at=now() WHERE house_id=$1 AND user_id=$2 AND role='RESIDENT' AND status='ACTIVE'`, r.str("house_id"), r.str("target_user_id"))
		if e != nil {
			return nil, e
		}
		if tag.RowsAffected() != 1 {
			return nil, conflict()
		}
		if e = emit(ctx, tx, a, "house.chairman.transfer_completed", r.str("house_id"), r.str("current_chairman_user_id"), c.ID); e != nil {
			return nil, e
		}
	}
	r, e = one(ctx, tx, `UPDATE chairman_transfers SET status=$2,completed_at=now() WHERE id=$1 RETURNING to_jsonb(chairman_transfers)`, c.ID, target)
	if e != nil {
		return nil, e
	}
	recipient := r.str("target_user_id")
	if target == "REJECTED" {
		recipient = r.str("current_chairman_user_id")
	}
	e = emit(ctx, tx, a, "house.chairman.transfer_"+map[string]string{"ACCEPTED": "accepted", "REJECTED": "rejected", "CANCELLED": "cancelled"}[target], r.str("house_id"), recipient, c.ID)
	return decorate(r, kind), e
}
