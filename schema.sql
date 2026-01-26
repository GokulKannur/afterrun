CREATE TABLE jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    ping_key VARCHAR(64) UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE job_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID REFERENCES jobs(id) ON DELETE CASCADE,
    status VARCHAR(20) NOT NULL,
    duration_ms INT,
    metrics JSONB,
    stderr TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID REFERENCES jobs(id) ON DELETE CASCADE,
    metric_name VARCHAR(100) NOT NULL,
    operator VARCHAR(10) NOT NULL,
    threshold_value FLOAT NOT NULL,
    severity VARCHAR(20) DEFAULT 'critical',
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE alerts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID REFERENCES jobs(id),
    run_id UUID REFERENCES job_runs(id),
    message TEXT NOT NULL,
    sent_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_job_runs_job_id ON job_runs(job_id);
CREATE INDEX idx_job_runs_created_at ON job_runs(created_at DESC);

-- Test Data
INSERT INTO jobs (name, ping_key) 
VALUES ('Test Backup Job', 'test123');

INSERT INTO rules (job_id, metric_name, operator, threshold_value)
VALUES (
  (SELECT id FROM jobs WHERE ping_key = 'test123'),
  'rows_processed',
  '==',
  0
);
