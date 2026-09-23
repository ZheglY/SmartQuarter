-- +goose Up
-- +goose StatementBegin
CREATE FUNCTION identity_address_part(text) RETURNS text LANGUAGE sql IMMUTABLE STRICT AS $$
 SELECT trim(regexp_replace(replace(lower($1),'ё','е'),'[[:space:],.;]+',' ','g'))
$$;
CREATE FUNCTION identity_address_key(text,text) RETURNS text LANGUAGE sql IMMUTABLE STRICT AS $$
 SELECT identity_address_part($1)||'|'||identity_address_part($2)
$$;
ALTER TABLE houses ADD COLUMN normalized_address text GENERATED ALWAYS AS (identity_address_key(city,address)) STORED;
CREATE UNIQUE INDEX houses_normalized_address_unique ON houses(normalized_address);
CREATE UNIQUE INDEX memberships_one_active_chairman ON memberships(house_id) WHERE role='CHAIRMAN' AND status='ACTIVE';
CREATE TABLE house_registrations (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), applicant_user_id uuid NOT NULL REFERENCES users(id),
 requested_name varchar(255) NOT NULL, original_address varchar(1000) NOT NULL, city varchar(100) NOT NULL,
 normalized_address text NOT NULL, status text NOT NULL DEFAULT 'PENDING' CHECK(status IN('PENDING','APPROVED','REJECTED','CANCELLED')),
 resulting_house_id uuid REFERENCES houses(id), rejection_reason varchar(1000), reviewed_by uuid REFERENCES users(id),
 reviewed_at timestamptz, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX registrations_pending_address ON house_registrations(normalized_address) WHERE status='PENDING';
CREATE INDEX registrations_applicant_status ON house_registrations(applicant_user_id,status);
CREATE TABLE house_invitations (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), house_id uuid NOT NULL REFERENCES houses(id), created_by uuid NOT NULL REFERENCES users(id),
 token_hash text UNIQUE NOT NULL, expires_at timestamptz NOT NULL, max_uses integer NOT NULL CHECK(max_uses BETWEEN 1 AND 100),
 used_count integer NOT NULL DEFAULT 0 CHECK(used_count>=0 AND used_count<=max_uses),
 status text NOT NULL DEFAULT 'ACTIVE' CHECK(status IN('ACTIVE','EXPIRED','REVOKED','EXHAUSTED')),
 created_at timestamptz NOT NULL DEFAULT now(), revoked_at timestamptz
);
CREATE TABLE join_requests (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), user_id uuid NOT NULL REFERENCES users(id), house_id uuid NOT NULL REFERENCES houses(id),
 source text NOT NULL CHECK(source IN('SEARCH','INVITE','ADMIN')), invite_id uuid REFERENCES house_invitations(id),
 status text NOT NULL DEFAULT 'PENDING' CHECK(status IN('PENDING','APPROVED','REJECTED','CANCELLED')),
 rejection_reason varchar(1000), reviewed_by uuid REFERENCES users(id), reviewed_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX join_requests_one_pending ON join_requests(user_id,house_id) WHERE status='PENDING';
CREATE INDEX join_requests_house_status ON join_requests(house_id,status);
CREATE TABLE invitation_redemptions (invite_id uuid NOT NULL REFERENCES house_invitations(id),user_id uuid NOT NULL REFERENCES users(id),request_id uuid NOT NULL REFERENCES join_requests(id),PRIMARY KEY(invite_id,user_id));
CREATE TABLE chairman_transfers (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), house_id uuid NOT NULL REFERENCES houses(id),
 current_chairman_user_id uuid NOT NULL REFERENCES users(id),target_user_id uuid NOT NULL REFERENCES users(id),
 status text NOT NULL DEFAULT 'PENDING' CHECK(status IN('PENDING','ACCEPTED','REJECTED','CANCELLED','EXPIRED')),
 expires_at timestamptz NOT NULL DEFAULT now()+interval '48 hours',created_at timestamptz NOT NULL DEFAULT now(),completed_at timestamptz,
 CHECK(current_chairman_user_id<>target_user_id)
);
CREATE UNIQUE INDEX chairman_transfers_one_pending ON chairman_transfers(house_id) WHERE status='PENDING';
CREATE TABLE notification_preferences (
 user_id uuid PRIMARY KEY REFERENCES users(id), notifications_enabled boolean NOT NULL DEFAULT true,
 issue_notifications_enabled boolean NOT NULL DEFAULT true, announcement_notifications_enabled boolean NOT NULL DEFAULT true,
 membership_notifications_enabled boolean NOT NULL DEFAULT true, bot_notifications_enabled boolean NOT NULL DEFAULT true
);
CREATE TABLE house_audit (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),actor_user_id uuid NOT NULL REFERENCES users(id),
 operation text NOT NULL, resource_id uuid,house_id uuid,created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE identity_outbox (
 event_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),event_type text NOT NULL,event_version integer NOT NULL DEFAULT 1,
 occurred_at timestamptz NOT NULL DEFAULT now(),producer text NOT NULL DEFAULT 'identity-service',payload jsonb NOT NULL,
 published_at timestamptz,attempts integer NOT NULL DEFAULT 0,last_error text
);
CREATE INDEX identity_outbox_unpublished ON identity_outbox(occurred_at) WHERE published_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE identity_outbox,house_audit,notification_preferences,chairman_transfers,invitation_redemptions,join_requests,house_invitations,house_registrations;
DROP INDEX memberships_one_active_chairman;
ALTER TABLE houses DROP COLUMN normalized_address;
DROP FUNCTION identity_address_key(text,text);
DROP FUNCTION identity_address_part(text);
-- +goose StatementEnd
