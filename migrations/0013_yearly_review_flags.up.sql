CREATE TABLE public.yearly_review_flags (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id             UUID NOT NULL REFERENCES public.runs(id) ON DELETE CASCADE,
    module_id          TEXT NOT NULL REFERENCES public.modules(id),
    flag_id            TEXT NOT NULL,
    severity           TEXT NOT NULL CHECK (severity IN ('minor','moderate','severe')),
    note               TEXT NOT NULL DEFAULT '',
    can_be_offset_by   JSONB NOT NULL DEFAULT '[]',
    is_offset          BOOLEAN NOT NULL DEFAULT false,
    offset_by_module_id TEXT,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (run_id, flag_id)
);
