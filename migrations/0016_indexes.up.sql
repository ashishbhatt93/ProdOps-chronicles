-- runs
CREATE INDEX idx_runs_player_id       ON public.runs (player_id);
-- module_progress
CREATE INDEX idx_mp_run_module        ON public.module_progress (run_id, module_id);
-- decisions_made
CREATE INDEX idx_decisions_run_module ON public.decisions_made (run_id, module_id);
-- tracker_snapshots
CREATE INDEX idx_snapshots_run_module ON public.tracker_snapshots (run_id, module_id);
-- task_completions
CREATE INDEX idx_tc_run_module        ON public.task_completions (run_id, module_id);
-- performance_scores
CREATE INDEX idx_perf_run             ON public.performance_scores (run_id);
-- yearly_review_flags
CREATE INDEX idx_flags_run            ON public.yearly_review_flags (run_id);
-- module_completions
CREATE INDEX idx_completions_run      ON public.module_completions (run_id, module_id);
