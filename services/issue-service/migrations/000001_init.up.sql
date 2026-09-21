CREATE TABLE issues (
 id UUID PRIMARY KEY,
 house_id UUID NOT NULL,
 created_by UUID NOT NULL,
 house_address_snapshot TEXT NOT NULL,
 category VARCHAR(32) NOT NULL CHECK (category IN ('SAFETY','CLEANLINESS','UTILITIES','INFRASTRUCTURE','OTHER')),
 description TEXT NOT NULL,
 location_text TEXT NOT NULL DEFAULT '',
 status VARCHAR(32) NOT NULL CHECK (status IN ('DETECTED','CONFIRMING','READY_FOR_APPEAL','HANDED_TO_CHAIRMAN','MARKED_SENT','WAITING_RESULT','RESOLVED')),
 confirmations_count INTEGER NOT NULL DEFAULT 0 CHECK (confirmations_count >= 0),
 created_at TIMESTAMPTZ NOT NULL,
 updated_at TIMESTAMPTZ NOT NULL,
 resolved_at TIMESTAMPTZ,
 CHECK ((status = 'RESOLVED') = (resolved_at IS NOT NULL))
);
CREATE TABLE attachments (
 id UUID PRIMARY KEY,
 house_id UUID NOT NULL,
 issue_id UUID REFERENCES issues(id),
 uploaded_by UUID NOT NULL,
 object_key TEXT NOT NULL UNIQUE,
 original_filename TEXT NOT NULL,
 mime_type VARCHAR(64) NOT NULL CHECK (mime_type IN ('image/jpeg','image/png')),
 size_bytes BIGINT NOT NULL CHECK (size_bytes > 0),
 sha256 CHAR(64),
 etag TEXT,
 status VARCHAR(16) NOT NULL CHECK (status IN ('UPLOADING','READY','ATTACHED','REJECTED','EXPIRED')),
 upload_expires_at TIMESTAMPTZ NOT NULL,
 created_at TIMESTAMPTZ NOT NULL,
 updated_at TIMESTAMPTZ NOT NULL,
 CHECK ((status = 'ATTACHED') = (issue_id IS NOT NULL)),
 CHECK (status NOT IN ('READY','ATTACHED') OR (sha256 IS NOT NULL AND etag IS NOT NULL))
);
CREATE TABLE confirmations (
 issue_id UUID NOT NULL REFERENCES issues(id),
 user_id UUID NOT NULL,
 created_at TIMESTAMPTZ NOT NULL,
 PRIMARY KEY(issue_id,user_id)
);
CREATE TABLE timeline_events (
 id UUID PRIMARY KEY,
 issue_id UUID NOT NULL REFERENCES issues(id),
 type VARCHAR(64) NOT NULL,
 actor_user_id UUID,
 payload JSONB NOT NULL DEFAULT '{}',
 created_at TIMESTAMPTZ NOT NULL
);
CREATE TABLE statement_drafts (
 id UUID PRIMARY KEY,
 issue_id UUID NOT NULL REFERENCES issues(id),
 version INTEGER NOT NULL CHECK (version > 0),
 status VARCHAR(16) NOT NULL CHECK (status = 'DRAFT'),
 body TEXT NOT NULL,
 chairman_note TEXT NOT NULL DEFAULT '',
 source_snapshot JSONB NOT NULL,
 created_by UUID NOT NULL,
 created_at TIMESTAMPTZ NOT NULL,
 updated_at TIMESTAMPTZ NOT NULL,
 UNIQUE(issue_id,version)
);
CREATE TABLE outbox_events (
 event_id UUID PRIMARY KEY,
 aggregate_type VARCHAR(32) NOT NULL,
 aggregate_id UUID NOT NULL,
 event_type VARCHAR(64) NOT NULL,
 event_version INTEGER NOT NULL CHECK (event_version > 0),
 payload JSONB NOT NULL,
 created_at TIMESTAMPTZ NOT NULL,
 published_at TIMESTAMPTZ,
 attempts INTEGER NOT NULL DEFAULT 0 CHECK(attempts >= 0),
 last_error TEXT,
 next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_issues_house_created ON issues(house_id,created_at DESC,id DESC);
CREATE INDEX idx_issues_house_status_created ON issues(house_id,status,created_at DESC,id DESC);
CREATE INDEX idx_attachments_issue ON attachments(issue_id);
CREATE INDEX idx_timeline_issue_created ON timeline_events(issue_id,created_at,id);
CREATE INDEX idx_statements_issue_version ON statement_drafts(issue_id,version DESC);
CREATE INDEX idx_outbox_pending ON outbox_events(created_at) WHERE published_at IS NULL;
CREATE INDEX idx_outbox_retry ON outbox_events(next_attempt_at,created_at) WHERE published_at IS NULL;
