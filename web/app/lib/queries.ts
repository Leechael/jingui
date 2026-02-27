import { queryOptions } from "@tanstack/react-query";
import { getClient } from "./api-client";

export const appKeys = {
  all: ["apps"] as const,
  detail: (appId: string) => ["apps", appId] as const,
};

export const instanceKeys = {
  all: ["instances"] as const,
  detail: (fid: string) => ["instances", fid] as const,
};

export const secretKeys = {
  all: ["secrets"] as const,
  detail: (vault: string, item: string) => ["secrets", vault, item] as const,
};

export const debugPolicyKeys = {
  detail: (vault: string, item: string) =>
    ["debug-policy", vault, item] as const,
};

export function appsQuery() {
  return queryOptions({
    queryKey: appKeys.all,
    queryFn: () => getClient().listApps(),
  });
}

export function appDetailQuery(appId: string) {
  return queryOptions({
    queryKey: appKeys.detail(appId),
    queryFn: () => getClient().getApp(appId),
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

export function secretsQuery() {
  return queryOptions({
    queryKey: secretKeys.all,
    queryFn: () => getClient().listSecrets(),
  });
}

export function secretDetailQuery(vault: string, item: string) {
  return queryOptions({
    queryKey: secretKeys.detail(vault, item),
    queryFn: () => getClient().getSecret(vault, item),
  });
}

export function debugPolicyQuery(vault: string, item: string) {
  return queryOptions({
    queryKey: debugPolicyKeys.detail(vault, item),
    queryFn: () => getClient().getDebugPolicy(vault, item),
  });
}
