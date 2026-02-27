import { useMutation, useQueryClient } from "@tanstack/react-query";
import { getClient } from "./api-client";
import { appKeys, instanceKeys, secretKeys, debugPolicyKeys } from "./queries";
import { addToast } from "./toast";
import type {
  AppRequest,
  InstanceRequest,
  InstanceUpdateRequest,
  CredentialsRequest,
  DebugPolicyRequest,
} from "./types";

// Apps
export function useCreateApp() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: AppRequest) => getClient().createApp(data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: appKeys.all });
      addToast("App created successfully");
    },
    onError: (e) => addToast(e.message, "error"),
  });
}

export function useUpdateApp(appId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: AppRequest) => getClient().updateApp(appId, data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: appKeys.all });
      qc.invalidateQueries({ queryKey: appKeys.detail(appId) });
      addToast("App updated successfully");
    },
    onError: (e) => addToast(e.message, "error"),
  });
}

export function useDeleteApp() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ appId, cascade }: { appId: string; cascade: boolean }) =>
      getClient().deleteApp(appId, cascade),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: appKeys.all });
      addToast("App deleted successfully");
    },
    onError: (e) => addToast(e.message, "error"),
  });
}

// Instances
export function useRegisterInstance() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: InstanceRequest) =>
      getClient().registerInstance(data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: instanceKeys.all });
      addToast("Instance registered successfully");
    },
    onError: (e) => addToast(e.message, "error"),
  });
}

export function useUpdateInstance(fid: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: InstanceUpdateRequest) =>
      getClient().updateInstance(fid, data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: instanceKeys.all });
      qc.invalidateQueries({ queryKey: instanceKeys.detail(fid) });
      addToast("Instance updated successfully");
    },
    onError: (e) => addToast(e.message, "error"),
  });
}

export function useDeleteInstance() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (fid: string) => getClient().deleteInstance(fid),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: instanceKeys.all });
      addToast("Instance deleted successfully");
    },
    onError: (e) => addToast(e.message, "error"),
  });
}

// Credentials
export function usePutCredentials(appId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: CredentialsRequest) =>
      getClient().putCredentials(appId, data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: appKeys.detail(appId) });
      qc.invalidateQueries({ queryKey: secretKeys.all });
      addToast("Credentials stored successfully");
    },
    onError: (e) => addToast(e.message, "error"),
  });
}

export function useStartDeviceFlow(appId: string) {
  return useMutation({
    mutationFn: () => getClient().startDeviceFlow(appId),
    onError: (e) => addToast(e.message, "error"),
  });
}

// Secrets
export function useDeleteSecret() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      vault,
      item,
      cascade,
    }: {
      vault: string;
      item: string;
      cascade: boolean;
    }) => getClient().deleteSecret(vault, item, cascade),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: secretKeys.all });
      addToast("Secret deleted successfully");
    },
    onError: (e) => addToast(e.message, "error"),
  });
}

// Debug Policy
export function useUpdateDebugPolicy(vault: string, item: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: DebugPolicyRequest) =>
      getClient().updateDebugPolicy(vault, item, data),
    onSuccess: (res) => {
      qc.invalidateQueries({
        queryKey: debugPolicyKeys.detail(vault, item),
      });
      addToast(
        `Debug read ${res.allow_read_debug ? "enabled" : "disabled"}`,
      );
    },
    onError: (e) => addToast(e.message, "error"),
  });
}
