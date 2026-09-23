package house

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"github.com/jackc/pgx/v5"
	"time"
)

func tokenHash(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
func invitationState(r record) record {
	if r.str("status") == "ACTIVE" && expired(r) {
		r["status"] = "EXPIRED"
	}
	delete(r, "token_hash")
	return decorate(r, "INVITATION_STATUS")
}
func (s *Service) invitation(ctx context.Context, tx pgx.Tx, a actor, op string, c Command) (record, error) {
	switch op {
	case "CreateHouseInvitation":
		if e := manager(ctx, tx, a, a.House); e != nil {
			return nil, e
		}
		if c.ExpiresInHours < 1 || c.ExpiresInHours > 168 || c.MaxUses < 1 || c.MaxUses > 100 {
			return nil, invalid()
		}
		bytes := make([]byte, 32)
		if _, e := rand.Read(bytes); e != nil {
			return nil, e
		}
		token := base64.RawURLEncoding.EncodeToString(bytes)
		r, e := one(ctx, tx, `INSERT INTO house_invitations(house_id,created_by,token_hash,expires_at,max_uses) VALUES($1,$2,$3,$4,$5) RETURNING to_jsonb(house_invitations)`, a.House, a.User, tokenHash(token), time.Now().Add(time.Duration(c.ExpiresInHours)*time.Hour), c.MaxUses)
		if e != nil {
			return nil, e
		}
		e = emit(ctx, tx, a, "house.invite.created", a.House, a.User, r.str("id"))
		return record{"invitation": invitationState(r), "token": token}, e
	case "ListHouseInvitations":
		if e := manager(ctx, tx, a, a.House); e != nil {
			return nil, e
		}
		rs, e := rows(ctx, tx, `SELECT to_jsonb(i) FROM house_invitations i WHERE house_id=$1 ORDER BY created_at DESC LIMIT 100`, a.House)
		for _, r := range rs {
			invitationState(r)
		}
		return list(rs, ""), e
	case "GetHouseInvitation", "RevokeHouseInvitation":
		if !validID(c.ID) {
			return nil, invalid()
		}
		r, e := one(ctx, tx, `SELECT to_jsonb(i) FROM house_invitations i WHERE id=$1 FOR UPDATE`, c.ID)
		if e != nil {
			return nil, e
		}
		if e = manager(ctx, tx, a, r.str("house_id")); e != nil {
			return nil, e
		}
		if op == "RevokeHouseInvitation" && r.str("status") != "REVOKED" {
			r, e = one(ctx, tx, `UPDATE house_invitations SET status='REVOKED',revoked_at=now() WHERE id=$1 RETURNING to_jsonb(house_invitations)`, c.ID)
			if e != nil {
				return nil, e
			}
			e = emit(ctx, tx, a, "house.invite.revoked", r.str("house_id"), a.User, c.ID)
		}
		return invitationState(r), e
	}
	decoded, e := base64.RawURLEncoding.DecodeString(c.Token)
	if e != nil || len(decoded) != 32 || base64.RawURLEncoding.EncodeToString(decoded) != c.Token {
		return nil, invalid()
	}
	r, e := one(ctx, tx, `SELECT to_jsonb(i) FROM house_invitations i WHERE token_hash=$1 FOR UPDATE`, tokenHash(c.Token))
	if e != nil {
		return nil, e
	}
	if op == "RedeemHouseInvitation" {
		old, e := one(ctx, tx, `SELECT to_jsonb(j) FROM invitation_redemptions d JOIN join_requests j ON j.id=d.request_id WHERE d.invite_id=$1 AND d.user_id=$2`, r.str("id"), a.User)
		if e == nil {
			return decorate(old, "JOIN_REQUEST_STATUS"), nil
		}
		if e != pgx.ErrNoRows {
			return nil, e
		}
	}
	if r.str("status") != "ACTIVE" || expired(r) {
		return nil, conflict()
	}
	if op == "PreviewHouseInvitation" {
		return houseSummary(ctx, tx, r.str("house_id"))
	}
	join, e := createJoin(ctx, tx, a, r.str("house_id"), "INVITE", r.str("id"))
	if e != nil {
		return nil, e
	}
	if _, e = tx.Exec(ctx, `INSERT INTO invitation_redemptions(invite_id,user_id,request_id) VALUES($1,$2,$3)`, r.str("id"), a.User, join.str("id")); e != nil {
		return nil, e
	}
	tag, e := tx.Exec(ctx, `UPDATE house_invitations SET used_count=used_count+1,status=CASE WHEN used_count+1>=max_uses THEN 'EXHAUSTED' ELSE 'ACTIVE' END WHERE id=$1 AND used_count<max_uses`, r.str("id"))
	if e != nil {
		return nil, e
	}
	if tag.RowsAffected() != 1 {
		return nil, conflict()
	}
	e = emit(ctx, tx, a, "house.invite.redeemed", r.str("house_id"), a.User, r.str("id"))
	return decorate(join, "JOIN_REQUEST_STATUS"), e
}
