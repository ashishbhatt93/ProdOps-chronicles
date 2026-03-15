CREATE TABLE public.module_completions (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id         UUID NOT NULL REFERENCES public.runs(id) ON DELETE CASCADE,
    module_id      TEXT NOT NULL REFERENCES public.modules(id),
    attempt_number INT  NOT NULL,
    ending_id      TEXT NOT NULL,
    final_score    INT  NOT NULL,
    completed_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    tracker_state  JSONB NOT NULL DEFAULT '{}',
    UNIQUE (run_id, module_id, attempt_number)
);
