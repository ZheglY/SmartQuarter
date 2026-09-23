package house

import (
	"context"
	"github.com/jackc/pgx/v5"
	"strings"
)

func (s *Service) registration(ctx context.Context, tx pgx.Tx, a actor, op string, c Command) (record, error) {
	const kind = "HOUSE_REGISTRATION_STATUS"
	switch op {
	case "SearchHouses":
		if !textOK(c.Query, 255) {
			return nil, invalid()
		}
		// POSITION treats user input literally, including SQL wildcard characters.
		rs, e := rows(ctx, tx, `SELECT jsonb_build_object('id',h.id,'name',h.name,'address',h.address,'city',h.city,'has_chairman',EXISTS(SELECT 1 FROM memberships m WHERE m.house_id=h.id AND m.role='CHAIRMAN' AND m.status='ACTIVE'),'join_available',NOT EXISTS(SELECT 1 FROM memberships m WHERE m.house_id=h.id AND m.user_id=$2 AND m.status='ACTIVE')) FROM houses h WHERE position(lower($1) in lower(h.city||' '||h.address||' '||h.name))>0 ORDER BY h.city,h.address,h.id LIMIT 100`, strings.TrimSpace(c.Query), a.User)
		return list(rs, ""), e
	case "CreateHouseRegistration":
		if !textOK(c.Name, 255) || !textOK(c.Address, 1000) || !textOK(c.City, 100) {
			return nil, invalid()
		}
		key := NormalizeAddress(c.City, c.Address)
		var exists bool
		if e := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM houses WHERE normalized_address=$1)`, key).Scan(&exists); e != nil {
			return nil, e
		}
		if exists {
			return nil, conflict()
		}
		r, e := one(ctx, tx, `INSERT INTO house_registrations(applicant_user_id,requested_name,original_address,city,normalized_address) VALUES($1,$2,$3,$4,$5) RETURNING to_jsonb(house_registrations)`, a.User, strings.TrimSpace(c.Name), strings.TrimSpace(c.Address), strings.TrimSpace(c.City), key)
		if e != nil {
			return nil, e
		}
		e = emit(ctx, tx, a, "house.registration.created", "", a.User, r.str("id"))
		return decorate(r, kind), e
	case "ListMyHouseRegistrations":
		rs, e := rows(ctx, tx, `SELECT to_jsonb(r) FROM house_registrations r WHERE applicant_user_id=$1 ORDER BY created_at DESC LIMIT 100`, a.User)
		return list(rs, kind), e
	case "ListPendingHouseRegistrations":
		if !a.Admin {
			return nil, denied()
		}
		rs, e := rows(ctx, tx, `SELECT to_jsonb(r) FROM house_registrations r WHERE status='PENDING' ORDER BY created_at LIMIT 100`)
		return list(rs, kind), e
	}
	if !validID(c.ID) {
		return nil, invalid()
	}
	r, e := one(ctx, tx, `SELECT to_jsonb(r) FROM house_registrations r WHERE id=$1 FOR UPDATE`, c.ID)
	if e != nil {
		return nil, e
	}
	if op == "GetHouseRegistration" {
		if r.str("applicant_user_id") != a.User && !a.Admin {
			return nil, denied()
		}
		return decorate(r, kind), nil
	}
	if op == "CancelHouseRegistration" {
		if r.str("applicant_user_id") != a.User {
			return nil, denied()
		}
	} else if !a.Admin {
		return nil, denied()
	}
	target := shortStatus(op)
	if e = transition(r.str("status"), target); e != nil {
		return nil, e
	}
	if r.str("status") == target {
		return decorate(r, kind), nil
	}
	houseID := ""
	if target == "APPROVED" {
		e = tx.QueryRow(ctx, `INSERT INTO houses(name,address,city) VALUES($1,$2,$3) RETURNING id::text`, r.str("requested_name"), r.str("original_address"), r.str("city")).Scan(&houseID)
		if e != nil {
			return nil, e
		}
		var memberID string
		e = tx.QueryRow(ctx, `INSERT INTO memberships(user_id,house_id,role,status) VALUES($1,$2,'CHAIRMAN','ACTIVE') RETURNING id::text`, r.str("applicant_user_id"), houseID).Scan(&memberID)
		if e != nil {
			return nil, e
		}
		if _, e = tx.Exec(ctx, `UPDATE users SET default_house_id=COALESCE(default_house_id,$2::uuid),updated_at=now() WHERE id=$1`, r.str("applicant_user_id"), houseID); e != nil {
			return nil, e
		}
		for _, ev := range []string{"house.created", "house.chairman.assigned", "house.membership.created"} {
			if e = emit(ctx, tx, a, ev, houseID, r.str("applicant_user_id"), memberID); e != nil {
				return nil, e
			}
		}
	}
	r, e = one(ctx, tx, `UPDATE house_registrations SET status=$2,resulting_house_id=NULLIF($3,'')::uuid,rejection_reason=$4,reviewed_by=$5,reviewed_at=now() WHERE id=$1 RETURNING to_jsonb(house_registrations)`, c.ID, target, houseID, c.Reason, a.User)
	if e != nil {
		return nil, e
	}
	e = emit(ctx, tx, a, "house.registration."+strings.ToLower(target), houseID, r.str("applicant_user_id"), c.ID)
	return decorate(r, kind), e
}

func (s *Service) join(ctx context.Context, tx pgx.Tx, a actor, op string, c Command) (record, error) {
	const kind = "JOIN_REQUEST_STATUS"
	switch op {
	case "ListMyJoinRequests":
		rs, e := rows(ctx, tx, `SELECT to_jsonb(r) FROM join_requests r WHERE user_id=$1 ORDER BY created_at DESC LIMIT 100`, a.User)
		return list(rs, kind), e
	case "ListHouseJoinRequests":
		if e := manager(ctx, tx, a, a.House); e != nil {
			return nil, e
		}
		rs, e := rows(ctx, tx, `SELECT to_jsonb(r) FROM join_requests r WHERE house_id=$1 ORDER BY created_at DESC LIMIT 100`, a.House)
		return list(rs, kind), e
	case "CreateJoinRequest":
		r, e := createJoin(ctx, tx, a, c.HouseID, "SEARCH", "")
		return decorate(r, kind), e
	}
	if !validID(c.ID) {
		return nil, invalid()
	}
	r, e := one(ctx, tx, `SELECT to_jsonb(r) FROM join_requests r WHERE id=$1 FOR UPDATE`, c.ID)
	if e != nil {
		return nil, e
	}
	if op == "CancelJoinRequest" {
		if r.str("user_id") != a.User {
			return nil, denied()
		}
	} else if e = manager(ctx, tx, a, r.str("house_id")); e != nil {
		return nil, e
	}
	target := shortStatus(op)
	if e = transition(r.str("status"), target); e != nil {
		return nil, e
	}
	if r.str("status") == target {
		return decorate(r, kind), nil
	}
	if target == "APPROVED" {
		var memberID string
		// Reactivation never restores an old chairman role.
		e = tx.QueryRow(ctx, `INSERT INTO memberships(user_id,house_id,role,status) VALUES($1,$2,'RESIDENT','ACTIVE') ON CONFLICT(user_id,house_id) DO UPDATE SET role=CASE WHEN memberships.status='ACTIVE' THEN memberships.role ELSE 'RESIDENT' END,status='ACTIVE',updated_at=now() RETURNING id::text`, r.str("user_id"), r.str("house_id")).Scan(&memberID)
		if e != nil {
			return nil, e
		}
		if _, e = tx.Exec(ctx, `UPDATE users SET default_house_id=COALESCE(default_house_id,$2::uuid),updated_at=now() WHERE id=$1`, r.str("user_id"), r.str("house_id")); e != nil {
			return nil, e
		}
		if e = emit(ctx, tx, a, "house.membership.activated", r.str("house_id"), r.str("user_id"), memberID); e != nil {
			return nil, e
		}
	}
	r, e = one(ctx, tx, `UPDATE join_requests SET status=$2,rejection_reason=$3,reviewed_by=$4,reviewed_at=now() WHERE id=$1 RETURNING to_jsonb(join_requests)`, c.ID, target, c.Reason, a.User)
	if e != nil {
		return nil, e
	}
	e = emit(ctx, tx, a, "house.join."+strings.ToLower(target), r.str("house_id"), r.str("user_id"), c.ID)
	return decorate(r, kind), e
}

func createJoin(ctx context.Context, tx pgx.Tx, a actor, house, source, invite string) (record, error) {
	if !validID(house) {
		return nil, invalid()
	}
	if yes, e := active(ctx, tx, a.User, house); e != nil {
		return nil, e
	} else if yes {
		return nil, conflict()
	}
	r, e := one(ctx, tx, `SELECT to_jsonb(r) FROM join_requests r WHERE user_id=$1 AND house_id=$2 AND status='PENDING'`, a.User, house)
	if e == nil {
		return r, nil
	}
	if e != pgx.ErrNoRows {
		return nil, e
	}
	r, e = one(ctx, tx, `INSERT INTO join_requests(user_id,house_id,source,invite_id) VALUES($1,$2,$3,NULLIF($4,'')::uuid) RETURNING to_jsonb(join_requests)`, a.User, house, source, invite)
	if e != nil {
		return nil, e
	}
	e = emit(ctx, tx, a, "house.join.created", house, a.User, r.str("id"))
	return r, e
}
