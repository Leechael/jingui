import { useMutation, useQueryClient } from "@tanstack/react-query";
import { getClient } from "./api-client";
import {
  vaultKeys,
  vaultItemKeys,
  instanceKeys,
  debugPolicyKeys,
} from "./queries";
import { addToast } from "./toast";
import type {
  CreateVaultRequest,
  UpdateVaultRequest,
  InstanceRequest,
  InstanceUpdateRequest,
  DebugPolicyRequest,
} from "./types";

// Vaults
export function useCreateVault() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: CreateVaultRequest) => getClient().createVault(data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: vaultKeys.all });
      addToast("Vault created successfully");
    },
    onError: (e) => addToast(e.message, "error"),
  });
}

export function useUpdateVault(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: UpdateVaultRequest) =>
      getClient().updateVault(id, data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: vaultKeys.all });
      qc.invalidateQueries({ queryKey: vaultKeys.detail(id) });
      addToast("Vault updated successfully");
    },
    onError: (e) => addToast(e.message, "error"),
  });
}

export function useDeleteVault() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, cascade }: { id: string; cascade: boolean }) =>
      getClient().deleteVault(id, cascade),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: vaultKeys.all });
      addToast("Vault deleted successfully");
    },
    onError: (e) => addToast(e.message, "error"),
  });
}

// Vault Items
export function usePutItem(vaultId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      section,
      fields,
      delete: deleteKeys,
    }: {
      section: string;
      fields: Record<string, string>;
      delete?: string[];
    }) => getClient().putItem(vaultId, section, fields, deleteKeys),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: vaultItemKeys.all(vaultId) });
      addToast("Item saved successfully");
    },
    onError: (e) => addToast(e.message, "error"),
  });
}

export function useDeleteItem(vaultId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (section: string) => getClient().deleteItem(vaultId, section),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: vaultItemKeys.all(vaultId) });
      addToast("Item deleted successfully");
    },
    onError: (e) => addToast(e.message, "error"),
  });
}

// Vault ↔ Instance Access
export function useGrantVaultAccess(vaultId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (fid: string) => getClient().grantVaultAccess(vaultId, fid),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: instanceKeys.byVault(vaultId) });
      addToast("Access granted successfully");
    },
    onError: (e) => addToast(e.message, "error"),
  });
}

export function useRevokeVaultAccess(vaultId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (fid: string) => getClient().revokeVaultAccess(vaultId, fid),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: instanceKeys.byVault(vaultId) });
      addToast("Access revoked successfully");
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

// Debug Policy
export function useUpdateDebugPolicy(vaultId: string, fid: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: DebugPolicyRequest) =>
      getClient().updateDebugPolicy(vaultId, fid, data),
    onSuccess: (res) => {
      qc.invalidateQueries({
        queryKey: debugPolicyKeys.detail(vaultId, fid),
      });
      addToast(
        `Debug read ${res.allow_read ? "enabled" : "disabled"}`,
      );
    },
    onError: (e) => addToast(e.message, "error"),
  });
}
