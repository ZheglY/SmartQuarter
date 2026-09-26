BEGIN;
CREATE UNIQUE INDEX IF NOT EXISTS poll_options_id_poll ON poll_options(id,poll_id);
DO $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='poll_vote_option_belongs_to_poll') THEN
  ALTER TABLE poll_votes ADD CONSTRAINT poll_vote_option_belongs_to_poll
    FOREIGN KEY(option_id,poll_id) REFERENCES poll_options(id,poll_id) NOT VALID;
 END IF;
END $$;
ALTER TABLE poll_votes VALIDATE CONSTRAINT poll_vote_option_belongs_to_poll;
CREATE INDEX IF NOT EXISTS polls_house_created ON polls(house_id,created_at DESC,id DESC);
CREATE INDEX IF NOT EXISTS initiatives_house_created ON initiatives(house_id,created_at DESC,id DESC);
CREATE INDEX IF NOT EXISTS calendar_house_period ON calendar_events(house_id,starts_at,ends_at);
COMMIT;
