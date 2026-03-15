CREATE TABLE public.module_unlocks (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id        UUID NOT NULL REFERENCES public.runs(id) ON DELETE CASCADE,
    module_id     TEXT NOT NULL REFERENCES public.modules(id),
    unlocked_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    unlock_source TEXT NOT NULL CHECK (unlock_source IN ('earned','forced')),
    UNIQUE (run_id, module_id)
);
