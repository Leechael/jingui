import { queryOptions } from "@tanstack/react-query";
import { getClient } from "./api-client";

export const vaultKeys = {
  all: ["vaults"] as const,
  detail: (id: string) => ["vaults", id] as const,
};

export const vaultItemKeys = {
  all: (vaultId: string) => ["vaults", vaultId, "items"] as const,
  detail: (vaultId: string, section: string) =>
    ["vaults", vaultId, "items", section] as const,
};

export const instanceKeys = {
  all: ["instances"] as const,
  detail: (fid: string) => ["instances", fid] as const,
  byVault: (vaultId: string) => ["vaults", vaultId, "instances"] as const,
};

export const debugPolicyKeys = {
  detail: (vaultId: string, fid: string) =>
    ["debug-policy", vaultId, fid] as const,
};

export function vaultsQuery() {
  return queryOptions({
    queryKey: vaultKeys.all,
    queryFn: () => getClient().listVaults(),
  });
}

export function vaultDetailQuery(id: string) {
  return queryOptions({
    queryKey: vaultKeys.detail(id),
    queryFn: () => getClient().getVault(id),
  });
}

export function vaultItemsQuery(vaultId: string) {
  return queryOptions({
    queryKey: vaultItemKeys.all(vaultId),
    queryFn: () => getClient().listItems(vaultId),
  });
}

export function vaultItemDetailQuery(vaultId: string, section: string) {
  return queryOptions({
    queryKey: vaultItemKeys.detail(vaultId, section),
    queryFn: () => getClient().getItem(vaultId, section),
  });
}

export function instancesQuery() {
  return queryOptions({
    queryKey: instanceKeys.all,
    queryFn: () => getClient().listInstances(),
  });
}

export function instanceDetailQuery(fid: string) {
  return queryOptions({
    queryKey: instanceKeys.detail(fid),
    queryFn: () => getClient().getInstance(fid),
  });
}

export function vaultInstancesQuery(vaultId: string) {
  return queryOptions({
    queryKey: instanceKeys.byVault(vaultId),
    queryFn: () => getClient().listVaultInstances(vaultId),
  });
}

export function debugPolicyQuery(vaultId: string, fid: string) {
  return queryOptions({
    queryKey: debugPolicyKeys.detail(vaultId, fid),
    queryFn: () => getClient().getDebugPolicy(vaultId, fid),
  });
}
