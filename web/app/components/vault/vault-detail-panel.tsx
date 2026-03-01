import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { Plus, Server, Trash2, Vault, X } from "lucide-react";
import { vaultDetailQuery, vaultInstancesQuery } from "~/lib/queries";
import { useDeleteVault, useRevokeVaultAccess } from "~/lib/mutations";
import { ConfirmDeleteDialog } from "~/components/shared/confirm-delete-dialog";
import { DebugPolicyToggle } from "~/components/secrets/debug-policy-toggle";
import { AddInstanceDialog } from "~/components/vault/add-instance-dialog";
import { formatDateTime, truncate } from "~/lib/utils";

interface VaultDetailPanelProps {
  vault: string;
  onDeleted: () => void;
}

export function VaultDetailPanel({ vault, onDeleted }: VaultDetailPanelProps) {
  const { data: vaultData, isLoading } = useQuery(vaultDetailQuery(vault));
  const { data: instances, isLoading: isLoadingInstances } = useQuery(vaultInstancesQuery(vault));
  const deleteVault = useDeleteVault();
  const revokeVaultAccess = useRevokeVaultAccess(vault);
  const [showDelete, setShowDelete] = useState(false);
  const [showAddInstance, setShowAddInstance] = useState(false);

  if (isLoading) {
    return (
      <div className="flex-1 p-6">
        <div className="space-y-4">
          <div className="h-6 w-48 animate-pulse rounded bg-muted" />
          <div className="h-4 w-32 animate-pulse rounded bg-muted" />
        </div>
      </div>
    );
  }

  if (!vaultData) return null;

  return (
    <div className="flex-1 overflow-y-auto p-6">
      <div className="space-y-6">
        <div className="flex items-start gap-3">
          <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-accent">
            <Vault className="h-5 w-5" />
          </div>
          <div>
            <h3 className="text-lg font-semibold">{vaultData.name}</h3>
            <p className="text-sm text-muted-foreground">{vaultData.id}</p>
          </div>
        </div>

        <div className="space-y-3 text-sm">
          <div className="flex items-center justify-between">
            <span className="text-muted-foreground">Instances</span>
            <span>{instances?.length ?? 0}</span>
          </div>
          <div className="flex items-center justify-between">
            <span className="text-muted-foreground">Created</span>
            <span>{formatDateTime(vaultData.created_at)}</span>
          </div>
        </div>

        <div className="space-y-3 border-t pt-4">
          <div className="flex items-center justify-between">
            <h4 className="text-sm font-medium">Linked Instances</h4>
            <button
              onClick={() => setShowAddInstance(true)}
              className="rounded-md p-1 hover:bg-accent"
            >
              <Plus className="h-4 w-4" />
            </button>
          </div>
          {isLoadingInstances ? (
            <div className="space-y-2">
              <div className="h-12 animate-pulse rounded bg-muted" />
              <div className="h-12 animate-pulse rounded bg-muted" />
            </div>
          ) : instances && instances.length > 0 ? (
            instances.map((inst) => (
              <div key={inst.fid} className="space-y-2 rounded-md border p-3">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2 min-w-0">
                    <Server className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                    <div className="min-w-0">
                      <p className="text-xs font-medium">
                        {inst.label || truncate(inst.fid, 20)}
                      </p>
                      <code className="text-[11px] text-muted-foreground break-all">
                        {inst.dstack_app_id}
                      </code>
                    </div>
                  </div>
                  <button
                    onClick={() => revokeVaultAccess.mutate(inst.fid)}
                    className="rounded-md p-1 shrink-0 text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
                  >
                    <X className="h-3.5 w-3.5" />
                  </button>
                </div>
                <DebugPolicyToggle vaultId={vault} fid={inst.fid} />
              </div>
            ))
          ) : (
            <p className="text-sm text-muted-foreground">No instances linked.</p>
          )}
        </div>

        <div className="border-t pt-4">
          <button
            onClick={() => setShowDelete(true)}
            className="flex items-center gap-2 rounded-md border border-destructive/50 px-3 py-2 text-sm text-destructive hover:bg-destructive/10"
          >
            <Trash2 className="h-4 w-4" />
            Delete Vault
          </button>
        </div>
      </div>

      <AddInstanceDialog
        open={showAddInstance}
        onClose={() => setShowAddInstance(false)}
        vaultId={vault}
        linkedFids={instances?.map((i) => i.fid) ?? []}
      />

      <ConfirmDeleteDialog
        open={showDelete}
        onClose={() => setShowDelete(false)}
        onConfirm={(cascade) =>
          deleteVault.mutate(
            { id: vault, cascade },
            {
              onSuccess: () => {
                setShowDelete(false);
                onDeleted();
              },
            },
          )
        }
        title="Delete Vault"
        description={`This will permanently delete the vault "${vault}" and all its configuration.`}
        showCascade
        cascadeLabel="Also delete all items and instance access"
        isPending={deleteVault.isPending}
      />
    </div>
  );
}
