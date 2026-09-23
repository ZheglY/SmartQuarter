package house

import (
	"context"
	"github.com/jackc/pgx/v5"
)

// Applicant names are exposed only in already-authorized request responses.
func displayNames(ctx context.Context, tx pgx.Tx, data record) error {
	records := []record{data}
	if items, ok := data["items"].([]record); ok {
		records = items
	}
	ids := []string{}
	for _, r := range records {
		for _, key := range []string{"applicant_user_id", "user_id"} {
			if id := r.str(key); id != "" {
				ids = append(ids, id)
			}
		}
	}
	if len(ids) == 0 {
		return nil
	}
	rs, e := tx.Query(ctx, `SELECT id::text,display_name FROM users WHERE id=ANY($1::uuid[])`, ids)
	if e != nil {
		return e
	}
	defer rs.Close()
	names := map[string]string{}
	for rs.Next() {
		var id, name string
		if e = rs.Scan(&id, &name); e != nil {
			return e
		}
		names[id] = name
	}
	if e = rs.Err(); e != nil {
		return e
	}
	for _, r := range records {
		if id := r.str("applicant_user_id"); id != "" {
			r["applicant_display_name"] = names[id]
		}
		if id := r.str("user_id"); id != "" {
			r["display_name"] = names[id]
		}
	}
	return nil
}
