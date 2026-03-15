CREATE TABLE public.task_completions (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id           UUID NOT NULL REFERENCES public.runs(id) ON DELETE CASCADE,
    module_id        TEXT NOT NULL REFERENCES public.modules(id),
    act_id           TEXT NOT NULL,
    task_id          TEXT NOT NULL,
    attempt_count    INT  NOT NULL DEFAULT 1,
    first_passed_at  TIMESTAMPTZ,
    is_locked        BOOLEAN NOT NULL DEFAULT false,
    last_attempted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    xp_awarded       INT NOT NULL DEFAULT 0,
    check_results    JSONB,
    UNIQUE (run_id, task_id)
);
