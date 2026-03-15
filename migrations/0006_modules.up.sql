CREATE TABLE public.modules (
    id                TEXT PRIMARY KEY,
    name              TEXT NOT NULL,
    version           TEXT NOT NULL,
    pod_name          TEXT NOT NULL,
    order_index       INT  NOT NULL,
    mode              TEXT NOT NULL DEFAULT 'beginner',
    runtime           TEXT NOT NULL DEFAULT 'compose',
    description       TEXT NOT NULL DEFAULT '',
    requires_module_id TEXT REFERENCES public.modules(id),
    score_threshold   INT  NOT NULL DEFAULT 80
);
