import { getSettings } from "./settings";
import type {
  AppRequest,
  AppListItem,
  AppDetail,
  InstanceRequest,
  InstanceUpdateRequest,
  InstanceView,
  SecretListItem,
  SecretDetail,
  CredentialsRequest,
  DebugPolicyResponse,
  DebugPolicyRequest,
  DeviceFlowResponse,
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

  // Apps
  listApps() {
    return this.request<AppListItem[]>("/v1/apps");
  }

  getApp(appId: string) {
    return this.request<AppDetail>(`/v1/apps/${encodeURIComponent(appId)}`);
  }

  createApp(data: AppRequest) {
    return this.request<{ vault: string; status: string }>("/v1/apps", {
      method: "POST",
      body: JSON.stringify(data),
    });
  }

  updateApp(appId: string, data: AppRequest) {
    return this.request<{ vault: string; status: string }>(
      `/v1/apps/${encodeURIComponent(appId)}`,
      { method: "PUT", body: JSON.stringify(data) },
    );
  }

  deleteApp(appId: string, cascade = false) {
    const qs = cascade ? "?cascade=true" : "";
    return this.request<{ status: string; vault: string }>(
      `/v1/apps/${encodeURIComponent(appId)}${qs}`,
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

  // Secrets
  listSecrets(vault?: string) {
    const qs = vault ? `?vault=${encodeURIComponent(vault)}` : "";
    return this.request<SecretListItem[]>(`/v1/secrets${qs}`);
  }

  getSecret(vault: string, item: string) {
    return this.request<SecretDetail>(
      `/v1/secrets/${encodeURIComponent(vault)}/${encodeURIComponent(item)}`,
    );
  }

  deleteSecret(vault: string, item: string, cascade = false) {
    const qs = cascade ? "?cascade=true" : "";
    return this.request<{ status: string; vault: string; item: string }>(
      `/v1/secrets/${encodeURIComponent(vault)}/${encodeURIComponent(item)}${qs}`,
      { method: "DELETE" },
    );
  }

  // Credentials
  putCredentials(appId: string, data: CredentialsRequest) {
    return this.request<{ status: string; app_id: string; item: string }>(
      `/v1/credentials/${encodeURIComponent(appId)}`,
      { method: "PUT", body: JSON.stringify(data) },
    );
  }

  // Returns the gateway URL without embedding the token.
  // The server's gateway endpoint requires Bearer auth, which browser
  // redirects (window.open) cannot carry. Use the Device Flow tab
  // instead, or configure the server for cookie-based auth on this endpoint.
  getOAuthGatewayUrl(appId: string): string {
    return `${this.apiUrl}/v1/credentials/gateway/${encodeURIComponent(appId)}`;
  }

  startDeviceFlow(appId: string) {
    return this.request<DeviceFlowResponse>(
      `/v1/credentials/device/${encodeURIComponent(appId)}`,
      { method: "POST" },
    );
  }

  // Debug Policy
  getDebugPolicy(vault: string, item: string) {
    return this.request<DebugPolicyResponse>(
      `/v1/debug-policy/${encodeURIComponent(vault)}/${encodeURIComponent(item)}`,
    );
  }

  updateDebugPolicy(vault: string, item: string, data: DebugPolicyRequest) {
    return this.request<{
      status: string;
      vault: string;
      item: string;
      allow_read_debug: boolean;
    }>(
      `/v1/debug-policy/${encodeURIComponent(vault)}/${encodeURIComponent(item)}`,
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
