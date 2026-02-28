// Types for the new vault-centric schema

// Vault (replaces App)
export interface Vault {
  id: string;
  name: string;
  created_at: string;
}

export interface CreateVaultRequest {
  id: string;
  name: string;
}

export interface UpdateVaultRequest {
  name: string;
}

// TEE Instances
export interface InstanceView {
  fid: string;
  public_key: string;
  dstack_app_id: string;
  label: string;
  created_at: string;
  last_used_at: string | null;
}

export interface InstanceRequest {
  public_key: string;
  dstack_app_id: string;
  label?: string;
}

export interface InstanceUpdateRequest {
  dstack_app_id: string;
  label?: string;
}

// Debug Policy
export interface DebugPolicyRequest {
  allow_read: boolean;
}

export interface DebugPolicyResponse {
  vault_id: string;
  fid: string;
  allow_read: boolean;
  source?: string;
  updated_at?: string;
}

export interface ApiError {
  error: string;
  hint?: string;
}
