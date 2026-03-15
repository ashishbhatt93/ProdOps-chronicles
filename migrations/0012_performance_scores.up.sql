CREATE TABLE public.performance_scores (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id      UUID NOT NULL REFERENCES public.runs(id) ON DELETE CASCADE,
    module_id   TEXT NOT NULL REFERENCES public.modules(id),
    delta       INT  NOT NULL,
    reason      TEXT NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
