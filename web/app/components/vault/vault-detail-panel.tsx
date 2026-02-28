import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { Trash2, Vault } from "lucide-react";
import { vaultDetailQuery, vaultInstancesQuery } from "~/lib/queries";
import { useDeleteVault } from "~/lib/mutations";
import { ConfirmDeleteDialog } from "~/components/shared/confirm-delete-dialog";
import { formatDateTime } from "~/lib/utils";

interface VaultDetailPanelProps {
  vault: string;
  onDeleted: () => void;
}

export function VaultDetailPanel({ vault, onDeleted }: VaultDetailPanelProps) {
  const { data: vaultData, isLoading } = useQuery(vaultDetailQuery(vault));
  const { data: instances } = useQuery(vaultInstancesQuery(vault));
  const deleteVault = useDeleteVault();
  const [showDelete, setShowDelete] = useState(false);

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
