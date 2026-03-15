CREATE TABLE public.module_progress (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id            UUID NOT NULL REFERENCES public.runs(id) ON DELETE CASCADE,
    module_id         TEXT NOT NULL REFERENCES public.modules(id),
    status            TEXT NOT NULL DEFAULT 'not_started'
                          CHECK (status IN ('not_started','in_progress','completed')),
    current_act_id    TEXT,
    completed_acts    TEXT[] NOT NULL DEFAULT '{}',
    completed_tasks   TEXT[] NOT NULL DEFAULT '{}',
    morale            INT  NOT NULL DEFAULT 100,
    incident_severity TEXT NOT NULL DEFAULT 'P2',
    technical_debt    INT  NOT NULL DEFAULT 0,
    xp_earned         INT  NOT NULL DEFAULT 0,
    final_score       INT,
    ending_id         TEXT,
    replay_count      INT  NOT NULL DEFAULT 0,
    started_at        TIMESTAMPTZ,
    completed_at      TIMESTAMPTZ,
    last_active_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (run_id, module_id)
);
