// Types derived from docs/openapi.json schemas

export interface AppRequest {
  vault: string;
  name: string;
  service_type: string;
  required_scopes?: string;
  credentials_json: Record<string, unknown>;
}

export interface AppListItem {
  vault: string;
  name: string;
  service_type: string;
  required_scopes: string;
  created_at: string;
}

export interface AppDetail {
  vault: string;
  name: string;
  service_type: string;
  required_scopes: string;
  has_credentials: boolean;
  created_at: string;
}

export interface InstanceRequest {
  public_key: string;
  bound_vault: string;
  bound_attestation_app_id: string;
  bound_item: string;
  label?: string;
}

export interface InstanceUpdateRequest {
  bound_attestation_app_id: string;
  label?: string;
}

export interface InstanceView {
  fid: string;
  public_key: string;
  bound_vault: string;
  bound_attestation_app_id: string;
  bound_item: string;
  label: string;
  created_at: string;
  last_used_at: string | null;
}

export interface SecretListItem {
  vault: string;
  item: string;
  created_at: string;
  updated_at: string;
}

export interface SecretDetail {
  vault: string;
  item: string;
  has_secret: boolean;
  created_at: string;
  updated_at: string;
}

export interface CredentialsRequest {
  item: string;
  secrets: Record<string, string>;
}

export interface DebugPolicyRequest {
  allow_read_debug: boolean;
}

export interface DebugPolicyResponse {
  vault: string;
  item: string;
  allow_read_debug: boolean;
  source?: string;
  updated_at?: string;
}

export interface DeviceFlowResponse {
  user_code: string;
  verification_url: string;
  expires_in: number;
}

export interface ApiError {
  error: string;
  hint?: string;
}
