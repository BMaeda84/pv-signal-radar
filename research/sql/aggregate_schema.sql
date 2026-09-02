PRAGMA foreign_keys = ON;

CREATE TABLE disproportionality_cells (
    dataset_id TEXT NOT NULL,
    drug_text TEXT NOT NULL,
    drug_text_source TEXT NOT NULL CHECK (drug_text_source IN ('PROD_AI', 'DRUGNAME')),
    drug_role TEXT NOT NULL,
    event_pt TEXT NOT NULL,
    a INTEGER NOT NULL CHECK (a >= 0),
    b INTEGER NOT NULL CHECK (b >= 0),
    c INTEGER NOT NULL CHECK (c >= 0),
    d INTEGER NOT NULL CHECK (d >= 0),
    drug_reports INTEGER NOT NULL CHECK (drug_reports = a + b),
    event_reports INTEGER NOT NULL CHECK (event_reports = a + c),
    universe_reports INTEGER NOT NULL CHECK (universe_reports = a + b + c + d),
    comparator TEXT NOT NULL,
    event_scope TEXT NOT NULL,
    deduplication_policy TEXT NOT NULL,
    PRIMARY KEY (dataset_id, drug_text, drug_text_source, drug_role, event_pt)
) STRICT, WITHOUT ROWID;

CREATE INDEX disproportionality_cells_event_idx
    ON disproportionality_cells (dataset_id, event_pt);
