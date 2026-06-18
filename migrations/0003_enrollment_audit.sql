CREATE TABLE enrollment_keys (
  id BIGSERIAL PRIMARY KEY,
  tenant_id BIGINT REFERENCES tenants(id) ON DELETE SET NULL,
  headscale_key_id TEXT,
  key_hash TEXT NOT NULL,
  user_name TEXT NOT NULL,
  tags JSONB NOT NULL DEFAULT '[]'::jsonb,
  reusable BOOLEAN NOT NULL DEFAULT false,
  ephemeral BOOLEAN NOT NULL DEFAULT false,
  expires_at TIMESTAMPTZ,
  created_by TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  revoked_at TIMESTAMPTZ
);

CREATE INDEX enrollment_keys_tenant_created_idx ON enrollment_keys (tenant_id, created_at DESC);
CREATE INDEX enrollment_keys_user_name_idx ON enrollment_keys (user_name);
