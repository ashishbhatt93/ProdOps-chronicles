CREATE TABLE public.tracker_snapshots (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id            UUID NOT NULL REFERENCES public.runs(id) ON DELETE CASCADE,
    module_id         TEXT NOT NULL REFERENCES public.modules(id),
    act_id            TEXT NOT NULL,
    decision_id       UUID NOT NULL REFERENCES public.decisions_made(id) ON DELETE CASCADE,
    state             JSONB NOT NULL,
    snapshot_taken_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
