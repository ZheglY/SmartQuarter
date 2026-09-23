package contacts

import (
	"context"
	"encoding/json"
	"errors"
	"net/mail"
	"net/url"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Service struct{ DB *pgxpool.Pool }
type Input struct {
	Category         string `json:"category"`
	Title            string `json:"title"`
	OrganizationName string `json:"organization_name"`
	Phone            string `json:"phone"`
	AdditionalPhone  string `json:"additional_phone"`
	Email            string `json:"email"`
	Website          string `json:"website"`
	Description      string `json:"description"`
	Emergency        bool   `json:"emergency"`
	SortOrder        int32  `json:"sort_order"`
}

var phonePattern = regexp.MustCompile(`^\+[1-9][0-9]{7,14}$`)

func Validate(v *Input) error {
	if v == nil {
		return status.Error(codes.InvalidArgument, "contact required")
	}
	categories := map[string]bool{}
	for _, c := range strings.Split("MANAGEMENT_COMPANY HOA EMERGENCY_DISPATCH ELECTRICITY WATER HEATING GAS ELEVATOR WASTE INTERNET SECURITY OTHER", " ") {
		categories[c] = true
	}
	if !categories[v.Category] || strings.TrimSpace(v.Title) == "" || !phonePattern.MatchString(v.Phone) || (v.AdditionalPhone != "" && !phonePattern.MatchString(v.AdditionalPhone)) || v.SortOrder < 0 || v.SortOrder > 10000 {
		return status.Error(codes.InvalidArgument, "invalid contact")
	}
	for _, field := range []struct {
		value string
		limit int
	}{{v.Title, 150}, {v.OrganizationName, 255}, {v.Email, 254}, {v.Website, 2048}, {v.Description, 2000}} {
		if len([]rune(field.value)) > field.limit || strings.ContainsAny(field.value, "<>\x00") {
			return status.Error(codes.InvalidArgument, "invalid contact text")
		}
	}
	if v.Email != "" {
		a, e := mail.ParseAddress(v.Email)
		if e != nil || a.Address != v.Email {
			return status.Error(codes.InvalidArgument, "invalid email")
		}
	}
	if v.Website != "" {
		u, e := url.Parse(v.Website)
		if e != nil || u.Hostname() == "" || u.User != nil || (u.Scheme != "https" && u.Scheme != "http") {
			return status.Error(codes.InvalidArgument, "invalid website")
		}
	}
	return nil
}
func validID(s string) bool {
	u, e := uuid.Parse(s)
	return e == nil && u != uuid.Nil && u.String() == s
}
func (s *Service) Execute(ctx context.Context, op, user, role, house, id string, v *Input, archived bool) (raw []byte, err error) {
	if !validID(user) || !validID(house) || (op != "Create" && op != "List" && !validID(id)) {
		return nil, status.Error(codes.InvalidArgument, "invalid context")
	}
	manager := role == "CHAIRMAN" || role == "ADMIN"
	if !manager && (op != "Get" && op != "List" || archived) {
		return nil, status.Error(codes.PermissionDenied, "chairman required")
	}
	if op == "Create" || op == "Update" {
		if err = Validate(v); err != nil {
			return nil, err
		}
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "contacts unavailable")
	}
	defer tx.Rollback(ctx)
	defer func() {
		if errors.Is(err, pgx.ErrNoRows) {
			err = status.Error(codes.NotFound, "contact not found")
		} else if err != nil {
			if _, ok := status.FromError(err); !ok {
				err = status.Error(codes.Internal, "contacts unavailable")
			}
		}
	}()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, house); err != nil {
		return nil, err
	}
	if op == "List" {
		err = tx.QueryRow(ctx, `SELECT jsonb_build_object('items',COALESCE(jsonb_agg(CASE WHEN $3 THEN to_jsonb(c) ELSE to_jsonb(c)-'created_by'-'created_at'-'updated_at' END ORDER BY c.emergency DESC,c.sort_order,c.title,c.id),'[]'::jsonb)) FROM service_contacts c WHERE house_id=$1 AND ($2 OR is_active)`, house, archived, manager).Scan(&raw)
	} else if op == "Get" {
		err = tx.QueryRow(ctx, `SELECT CASE WHEN $3 THEN to_jsonb(c) ELSE to_jsonb(c)-'created_by'-'created_at'-'updated_at' END FROM service_contacts c WHERE house_id=$1 AND id=$2 AND ($3 OR is_active)`, house, id, manager).Scan(&raw)
	} else {
		if op == "Create" || op == "Update" {
			var count int
			err = tx.QueryRow(ctx, `SELECT count(*) FROM service_contacts WHERE house_id=$1 AND category=$2 AND is_active AND id<>COALESCE(NULLIF($3,'')::uuid,'00000000-0000-0000-0000-000000000000'::uuid)`, house, v.Category, id).Scan(&count)
			if err != nil {
				return nil, err
			}
			if count >= 10 {
				return nil, status.Error(codes.ResourceExhausted, "category limit reached")
			}
		}
		switch op {
		case "Create":
			err = tx.QueryRow(ctx, `INSERT INTO service_contacts(house_id,category,title,organization_name,phone,additional_phone,email,website,description,emergency,sort_order,created_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING to_jsonb(service_contacts)`, house, v.Category, v.Title, v.OrganizationName, v.Phone, v.AdditionalPhone, v.Email, v.Website, v.Description, v.Emergency, v.SortOrder, user).Scan(&raw)
		case "Update":
			err = tx.QueryRow(ctx, `UPDATE service_contacts SET category=$3,title=$4,organization_name=$5,phone=$6,additional_phone=$7,email=$8,website=$9,description=$10,emergency=$11,sort_order=$12,updated_at=now() WHERE house_id=$1 AND id=$2 AND is_active RETURNING to_jsonb(service_contacts)`, house, id, v.Category, v.Title, v.OrganizationName, v.Phone, v.AdditionalPhone, v.Email, v.Website, v.Description, v.Emergency, v.SortOrder).Scan(&raw)
		case "Archive":
			err = tx.QueryRow(ctx, `UPDATE service_contacts SET is_active=false,updated_at=now() WHERE house_id=$1 AND id=$2 RETURNING to_jsonb(service_contacts)`, house, id).Scan(&raw)
		default:
			return nil, status.Error(codes.Unimplemented, "unknown operation")
		}
		if err != nil {
			return nil, err
		}
		var row struct {
			ID string `json:"id"`
		}
		if err = json.Unmarshal(raw, &row); err != nil {
			return nil, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO service_contact_audit(contact_id,house_id,actor_user_id,operation) VALUES($1,$2,$3,$4)`, row.ID, house, user, op); err != nil {
			return nil, err
		}
		payload, _ := json.Marshal(map[string]string{"house_id": house, "contact_id": row.ID})
		if _, err = tx.Exec(ctx, `INSERT INTO outbox_events(event_id,event_type,producer,payload) VALUES($1,$2,'community-service',$3)`, uuid.NewString(), "service_contact."+strings.ToLower(op), payload); err != nil {
			return nil, err
		}
	}
	if err != nil {
		return nil, err
	}
	err = tx.Commit(ctx)
	return raw, err
}
