CREATE TABLE public.decisions_made (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id          UUID NOT NULL REFERENCES public.runs(id) ON DELETE CASCADE,
    module_id       TEXT NOT NULL REFERENCES public.modules(id),
    act_id          TEXT NOT NULL,
    option_id       TEXT NOT NULL,
    decided_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    morale_delta    INT  NOT NULL DEFAULT 0,
    severity_change TEXT,
    debt_delta      INT  NOT NULL DEFAULT 0
);
-- Append-only: no UPDATE or DELETE granted on this table
