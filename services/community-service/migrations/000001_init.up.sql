CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS announcements (
                                           id UUID PRIMARY KEY,
                                           house_id UUID NOT NULL,
                                           author_user_id UUID NOT NULL,
                                           title VARCHAR(255) NOT NULL,
                                           body TEXT NOT NULL,
                                           status VARCHAR(50) NOT NULL DEFAULT 'PUBLISHED',
                                           published_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
                                           created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS polls (
                                   id UUID PRIMARY KEY,
                                   house_id UUID NOT NULL,
                                   author_user_id UUID NOT NULL,
                                   question TEXT NOT NULL,
                                   status VARCHAR(50) NOT NULL DEFAULT 'OPEN',
                                   ends_at TIMESTAMP WITH TIME ZONE NOT NULL,
                                   created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS poll_options (
                                          id UUID PRIMARY KEY,
                                          poll_id UUID NOT NULL REFERENCES polls(id) ON DELETE CASCADE,
                                          text TEXT NOT NULL,
                                          position INT NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS poll_votes (
                                        poll_id UUID NOT NULL REFERENCES polls(id) ON DELETE CASCADE,
                                        option_id UUID NOT NULL REFERENCES poll_options(id) ON DELETE CASCADE,
                                        user_id UUID NOT NULL,
                                        PRIMARY KEY (poll_id, user_id)
);

CREATE TABLE IF NOT EXISTS calendar_events (
                                             id UUID PRIMARY KEY,
                                             house_id UUID NOT NULL,
                                             created_by UUID NOT NULL,
                                             title VARCHAR(255) NOT NULL,
                                             description TEXT,
                                             starts_at TIMESTAMP WITH TIME ZONE NOT NULL,
                                             ends_at TIMESTAMP WITH TIME ZONE NOT NULL,
                                             created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS initiatives (
                                         id UUID PRIMARY KEY,
                                         house_id UUID NOT NULL,
                                         author_user_id UUID NOT NULL,
                                         title VARCHAR(255) NOT NULL,
                                         description TEXT NOT NULL,
                                         status VARCHAR(50) NOT NULL DEFAULT 'OPEN',
                                         supports_count INT NOT NULL DEFAULT 0,
                                         created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
                                         updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS initiative_supports (
                                                 initiative_id UUID NOT NULL REFERENCES initiatives(id) ON DELETE CASCADE,
                                                 user_id UUID NOT NULL,
                                                 PRIMARY KEY (initiative_id, user_id)
);

CREATE TABLE IF NOT EXISTS outbox_events (
                                           event_id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
                                           event_type VARCHAR(255) NOT NULL,
                                           event_version INT NOT NULL DEFAULT 1,
                                           occurred_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
                                           producer VARCHAR(255) NOT NULL,
                                           payload JSONB NOT NULL,
                                           published_at TIMESTAMP WITH TIME ZONE -- Добавлено пропущенное поле
);
