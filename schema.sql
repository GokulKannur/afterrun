-- Jobs Table
CREATE TABLE IF NOT EXISTS jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    ping_key VARCHAR(64) UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Phase 2 Columns (Idempotent)
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS schedule VARCHAR(100);
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS timezone VARCHAR(50) DEFAULT 'UTC';
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS grace_minutes INT DEFAULT 30;

-- Phase 3.5: Users Table
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    stripe_customer_id VARCHAR(255),
    subscription_tier VARCHAR(50) DEFAULT 'free',
    subscription_status VARCHAR(50) DEFAULT 'active',
    created_at TIMESTAMP DEFAULT NOW()
);

-- Phase 3.5: Job User Relationship
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES users(id) ON DELETE CASCADE;

-- Phase 3.5: Baselines
CREATE TABLE IF NOT EXISTS baselines (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID REFERENCES jobs(id) ON DELETE CASCADE,
    metric_name VARCHAR(100) NOT NULL,
    p50 DOUBLE PRECISION,
    p95 DOUBLE PRECISION,
    p99 DOUBLE PRECISION,
    sample_size INT,
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Job Runs Table
CREATE TABLE IF NOT EXISTS job_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID REFERENCES jobs(id) ON DELETE CASCADE,
    status VARCHAR(20) NOT NULL,
    duration_ms INT,
    metrics JSONB,
    stderr TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Rules Table
CREATE TABLE IF NOT EXISTS rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID REFERENCES jobs(id) ON DELETE CASCADE,
    metric_name VARCHAR(100) NOT NULL,
    operator VARCHAR(10) NOT NULL,
    threshold_value FLOAT NOT NULL,
    severity VARCHAR(20) DEFAULT 'critical',
    created_at TIMESTAMP DEFAULT NOW()
);

-- Alerts Table
CREATE TABLE IF NOT EXISTS alerts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID REFERENCES jobs(id),
    run_id UUID REFERENCES job_runs(id),
    message TEXT NOT NULL,
    sent_at TIMESTAMP DEFAULT NOW()
);

-- Indexes (Idempotent via IF NOT EXISTS)
CREATE INDEX IF NOT EXISTS idx_job_runs_job_id ON job_runs(job_id);
CREATE INDEX IF NOT EXISTS idx_job_runs_created_at ON job_runs(created_at DESC);

-- Phase 4: Data Migration (System User)
INSERT INTO users (email, password_hash, subscription_tier, subscription_status)
VALUES ('system@afterrun.internal', 'locked', 'unlimited', 'active')
ON CONFLICT (email) DO NOTHING;

-- Link Orphaned Jobs
UPDATE jobs
SET user_id = (SELECT id FROM users WHERE email = 'system@afterrun.internal')
WHERE user_id IS NULL;

-- Enforce Ownership
ALTER TABLE jobs ALTER COLUMN user_id SET NOT NULL;

-- Test Data (Only if empty, linked to system user)
INSERT INTO jobs (name, ping_key, user_id) 
SELECT 'Test Backup Job', 'test123', (SELECT id FROM users WHERE email = 'system@afterrun.internal')
WHERE NOT EXISTS (SELECT 1 FROM jobs WHERE ping_key = 'test123');

INSERT INTO rules (job_id, metric_name, operator, threshold_value)
SELECT id, 'rows_processed', '==', 0
FROM jobs WHERE ping_key = 'test123'
AND NOT EXISTS (SELECT 1 FROM rules WHERE metric_name = 'rows_processed' AND job_id = (SELECT id FROM jobs WHERE ping_key = 'test123'));
