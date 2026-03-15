CREATE TABLE public.runs (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    player_id               UUID NOT NULL,
    status                  TEXT NOT NULL DEFAULT 'in_progress'
                                CHECK (status IN ('in_progress','completed')),
    started_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at            TIMESTAMPTZ,
    final_performance_score INT
);
-- Enforces at most one in_progress run per player
CREATE UNIQUE INDEX runs_player_active_uq
    ON public.runs (player_id)
    WHERE status = 'in_progress';
