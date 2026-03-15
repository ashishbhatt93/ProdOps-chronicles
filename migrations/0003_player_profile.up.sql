CREATE TABLE private.player_profile (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    player_id         UUID NOT NULL REFERENCES private.player_identity(id) ON DELETE CASCADE,
    git_username      TEXT,
    git_email         TEXT,
    ssh_key_path      TEXT,
    sync_remote       TEXT,
    telemetry_consent TEXT CHECK (telemetry_consent IN ('tier1','tier2','tier3')),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
