import { getSettings } from "./settings";
import type {
  Vault,
  CreateVaultRequest,
  UpdateVaultRequest,
  InstanceRequest,
  InstanceUpdateRequest,
  InstanceView,
  DebugPolicyResponse,
  DebugPolicyRequest,
  ApiError,
} from "./types";

class JinguiClient {
  private apiUrl: string;
  private token: string;

  constructor(apiUrl: string, token: string) {
    this.apiUrl = apiUrl.replace(/\/+$/, "");
    this.token = token;
  }

  private async request<T>(
    path: string,
    options: RequestInit = {},
  ): Promise<T> {
    const res = await fetch(`${this.apiUrl}${path}`, {
      ...options,
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${this.token}`,
        ...options.headers,
      },
    });

    if (!res.ok) {
      const body = (await res.json().catch(() => ({
        error: `HTTP ${res.status}`,
      }))) as ApiError;
      throw new ApiClientError(
        body.error || `HTTP ${res.status}`,
        res.status,
        body.hint,
      );
    }

    if (res.status === 204) return undefined as T;
    return res.json() as Promise<T>;
  }

  // Liveness
  async ping(): Promise<void> {
    const res = await fetch(`${this.apiUrl}/`, {
      headers: { Authorization: `Bearer ${this.token}` },
    });
    if (!res.ok) throw new Error(`Server returned ${res.status}`);
  }

  // Vaults
  listVaults() {
    return this.request<Vault[]>("/v1/vaults");
  }

  getVault(id: string) {
    return this.request<Vault>(
      `/v1/vaults/${encodeURIComponent(id)}`,
    );
  }

  createVault(data: CreateVaultRequest) {
    return this.request<{ id: string; status: string }>("/v1/vaults", {
      method: "POST",
      body: JSON.stringify(data),
    });
  }

  updateVault(id: string, data: UpdateVaultRequest) {
    return this.request<{ id: string; status: string }>(
      `/v1/vaults/${encodeURIComponent(id)}`,
      { method: "PUT", body: JSON.stringify(data) },
    );
  }

  deleteVault(id: string, cascade = false) {
    const qs = cascade ? "?cascade=true" : "";
    return this.request<{ status: string; id: string }>(
      `/v1/vaults/${encodeURIComponent(id)}${qs}`,
      { method: "DELETE" },
    );
  }

  // Vault Items
  listItems(vaultId: string) {
    return this.request<string[]>(
      `/v1/vaults/${encodeURIComponent(vaultId)}/items`,
    );
  }

  getItem(vaultId: string, section: string) {
    return this.request<{
      vault_id: string;
      section: string;
      fields: Record<string, string>;
    }>(
      `/v1/vaults/${encodeURIComponent(vaultId)}/items/${encodeURIComponent(section)}`,
    );
  }

  putItem(vaultId: string, section: string, fields: Record<string, string>) {
    return this.request<{ status: string }>(
      `/v1/vaults/${encodeURIComponent(vaultId)}/items/${encodeURIComponent(section)}`,
      { method: "PUT", body: JSON.stringify({ fields }) },
    );
  }

  deleteItem(vaultId: string, section: string) {
    return this.request<{ status: string }>(
      `/v1/vaults/${encodeURIComponent(vaultId)}/items/${encodeURIComponent(section)}`,
      { method: "DELETE" },
    );
  }

  // Vault ↔ Instance Access
  listVaultInstances(vaultId: string) {
    return this.request<InstanceView[]>(
      `/v1/vaults/${encodeURIComponent(vaultId)}/instances`,
    );
  }

  grantVaultAccess(vaultId: string, fid: string) {
    return this.request<{ status: string }>(
      `/v1/vaults/${encodeURIComponent(vaultId)}/instances/${encodeURIComponent(fid)}`,
      { method: "POST" },
    );
  }

  revokeVaultAccess(vaultId: string, fid: string) {
    return this.request<{ status: string }>(
      `/v1/vaults/${encodeURIComponent(vaultId)}/instances/${encodeURIComponent(fid)}`,
      { method: "DELETE" },
    );
  }

  // Instances
  listInstances() {
    return this.request<InstanceView[]>("/v1/instances");
  }

  getInstance(fid: string) {
    return this.request<InstanceView>(
      `/v1/instances/${encodeURIComponent(fid)}`,
    );
  }

  registerInstance(data: InstanceRequest) {
    return this.request<{ fid: string; status: string }>("/v1/instances", {
      method: "POST",
      body: JSON.stringify(data),
    });
  }

  updateInstance(fid: string, data: InstanceUpdateRequest) {
    return this.request<{ status: string; fid: string }>(
      `/v1/instances/${encodeURIComponent(fid)}`,
      { method: "PUT", body: JSON.stringify(data) },
    );
  }

  deleteInstance(fid: string) {
    return this.request<{ status: string; fid: string }>(
      `/v1/instances/${encodeURIComponent(fid)}`,
      { method: "DELETE" },
    );
  }

  // Debug Policy
  getDebugPolicy(vaultId: string, fid: string) {
    return this.request<DebugPolicyResponse>(
      `/v1/debug-policy/${encodeURIComponent(vaultId)}/${encodeURIComponent(fid)}`,
    );
  }

  updateDebugPolicy(vaultId: string, fid: string, data: DebugPolicyRequest) {
    return this.request<{
      status: string;
      vault_id: string;
      fid: string;
      allow_read: boolean;
    }>(
      `/v1/debug-policy/${encodeURIComponent(vaultId)}/${encodeURIComponent(fid)}`,
      { method: "PUT", body: JSON.stringify(data) },
    );
  }
}

export class ApiClientError extends Error {
  status: number;
  hint?: string;
  constructor(message: string, status: number, hint?: string) {
    super(message);
    this.name = "ApiClientError";
    this.status = status;
    this.hint = hint;
  }
}

let cachedClient: JinguiClient | null = null;

export function getClient(): JinguiClient {
  const settings = getSettings();
  if (!settings) throw new Error("API not configured");
  if (
    cachedClient &&
    (cachedClient as unknown as { apiUrl: string }).apiUrl === settings.apiUrl
  ) {
    return cachedClient;
  }
  cachedClient = new JinguiClient(settings.apiUrl, settings.token);
  return cachedClient;
}

export function clearClientCache(): void {
  cachedClient = null;
}
