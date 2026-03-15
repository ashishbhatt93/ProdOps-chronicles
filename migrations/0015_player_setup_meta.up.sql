CREATE TABLE public.player_setup_meta (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id              UUID NOT NULL REFERENCES public.runs(id) ON DELETE CASCADE,
    telemetry_consent   TEXT CHECK (telemetry_consent IN ('tier1','tier2','tier3')),
    default_branch_name TEXT,
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (run_id)
);
