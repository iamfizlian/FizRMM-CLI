CREATE TABLE tenants (
  id BIGSERIAL PRIMARY KEY,
  name TEXT NOT NULL,
  slug TEXT NOT NULL UNIQUE,
  headscale_instance_id BIGINT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE sites (
  id BIGSERIAL PRIMARY KEY,
  tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  timezone TEXT NOT NULL DEFAULT 'UTC',
  tags JSONB NOT NULL DEFAULT '[]'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE headscale_instances (
  id BIGSERIAL PRIMARY KEY,
  tenant_id BIGINT REFERENCES tenants(id) ON DELETE CASCADE,
  base_url TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'unknown',
  version TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE tenants
  ADD CONSTRAINT tenants_headscale_instance_id_fkey
  FOREIGN KEY (headscale_instance_id) REFERENCES headscale_instances(id);

CREATE TABLE nodes (
  id BIGSERIAL PRIMARY KEY,
  tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  site_id BIGINT REFERENCES sites(id) ON DELETE SET NULL,
  hostname TEXT NOT NULL,
  fqdn TEXT,
  os_family TEXT,
  os_version TEXT,
  architecture TEXT,
  headscale_node_id TEXT,
  tailnet_ip INET,
  last_seen_at TIMESTAMPTZ,
  status TEXT NOT NULL DEFAULT 'unknown',
  tags JSONB NOT NULL DEFAULT '[]'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX nodes_tenant_status_idx ON nodes (tenant_id, status);
CREATE INDEX nodes_headscale_node_id_idx ON nodes (headscale_node_id);

CREATE TABLE jobs (
  id BIGSERIAL PRIMARY KEY,
  tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  created_by TEXT NOT NULL,
  type TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'queued',
  target_selector TEXT NOT NULL,
  command_or_playbook TEXT NOT NULL,
  reason TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  started_at TIMESTAMPTZ,
  finished_at TIMESTAMPTZ
);

CREATE INDEX jobs_tenant_status_idx ON jobs (tenant_id, status);

CREATE TABLE job_results (
  id BIGSERIAL PRIMARY KEY,
  job_id BIGINT NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
  node_id BIGINT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
  exit_code INTEGER,
  stdout_ref TEXT,
  stderr_ref TEXT,
  started_at TIMESTAMPTZ,
  finished_at TIMESTAMPTZ
);

CREATE TABLE alerts (
  id BIGSERIAL PRIMARY KEY,
  tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  source TEXT NOT NULL,
  labels JSONB NOT NULL DEFAULT '{}'::jsonb,
  severity TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'active',
  starts_at TIMESTAMPTZ NOT NULL,
  ends_at TIMESTAMPTZ
);

CREATE INDEX alerts_tenant_status_idx ON alerts (tenant_id, status);

CREATE TABLE audit_events (
  id BIGSERIAL PRIMARY KEY,
  tenant_id BIGINT REFERENCES tenants(id) ON DELETE SET NULL,
  actor TEXT NOT NULL,
  action TEXT NOT NULL,
  target_type TEXT NOT NULL,
  target_id TEXT,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX audit_events_tenant_created_idx ON audit_events (tenant_id, created_at DESC);

