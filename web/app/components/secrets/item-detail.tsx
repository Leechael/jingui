import { useQuery } from "@tanstack/react-query";
import { useState, Suspense } from "react";
import { KeyRound, Trash2 } from "lucide-react";
import { secretDetailQuery } from "~/lib/queries";
import { useDeleteSecret } from "~/lib/mutations";
import { DebugPolicyToggle } from "./debug-policy-toggle";
import { StoreCredentialsForm } from "./store-credentials-form";
import { ConfirmDeleteDialog } from "~/components/shared/confirm-delete-dialog";
import { formatDateTime } from "~/lib/utils";

interface ItemDetailProps {
  vault: string;
  item: string;
  onDeleted: () => void;
}

export function ItemDetail({ vault, item, onDeleted }: ItemDetailProps) {
  const { data: detail, isLoading } = useQuery(
    secretDetailQuery(vault, item),
  );
  const deleteSecret = useDeleteSecret();
  const [showStore, setShowStore] = useState(false);
  const [showDelete, setShowDelete] = useState(false);

  if (isLoading) {
    return (
      <div className="flex-1 p-6">
        <div className="space-y-4">
          <div className="h-6 w-48 animate-pulse rounded bg-muted" />
          <div className="h-4 w-32 animate-pulse rounded bg-muted" />
          <div className="h-4 w-40 animate-pulse rounded bg-muted" />
        </div>
      </div>
    );
  }

  if (!detail) return null;

  return (
    <div className="flex-1 overflow-y-auto p-6">
      <div className="space-y-6">
        <div>
          <p className="text-xs font-medium uppercase text-muted-foreground">
            Vault
          </p>
          <p className="text-lg font-semibold">{detail.vault}</p>
        </div>

        <div>
          <p className="text-xs font-medium uppercase text-muted-foreground">
            Item
          </p>
          <p className="text-lg font-semibold">{detail.item}</p>
        </div>

        <div className="grid grid-cols-2 gap-4 text-sm">
          <div>
            <span className="text-muted-foreground">Has Secret</span>
            <p className="font-medium">
              {detail.has_secret ? (
                <span className="inline-flex items-center gap-1 rounded-full bg-green-100 px-2 py-0.5 text-xs text-green-700">
                  Yes
                </span>
              ) : (
                <span className="inline-flex items-center gap-1 rounded-full bg-muted px-2 py-0.5 text-xs text-muted-foreground">
                  No
                </span>
              )}
            </p>
          </div>
          <div>
            <span className="text-muted-foreground">Created</span>
            <p className="font-medium">{formatDateTime(detail.created_at)}</p>
          </div>
          <div>
            <span className="text-muted-foreground">Updated</span>
            <p className="font-medium">{formatDateTime(detail.updated_at)}</p>
          </div>
        </div>

        <div className="border-t pt-4">
          <DebugPolicyToggle vault={vault} item={item} />
        </div>

        <div className="flex gap-2 border-t pt-4">
          <button
            onClick={() => setShowStore(true)}
            className="flex items-center gap-2 rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90"
          >
            <KeyRound className="h-4 w-4" />
            Store Credentials
          </button>
          <button
            onClick={() => setShowDelete(true)}
            className="flex items-center gap-2 rounded-md border border-destructive/50 px-3 py-2 text-sm text-destructive hover:bg-destructive/10"
          >
            <Trash2 className="h-4 w-4" />
            Delete
          </button>
        </div>
      </div>

      <Suspense fallback={null}>
        <StoreCredentialsForm
          open={showStore}
          onClose={() => setShowStore(false)}
          defaultItem={item}
        />
      </Suspense>

      <ConfirmDeleteDialog
        open={showDelete}
        onClose={() => setShowDelete(false)}
        onConfirm={(cascade) =>
          deleteSecret.mutate(
            { vault, item, cascade },
            { onSuccess: () => { setShowDelete(false); onDeleted(); } },
          )
        }
        title="Delete Secret"
        description={`This will permanently delete "${vault}/${item}".`}
        showCascade
        cascadeLabel="Also delete dependent instances"
        isPending={deleteSecret.isPending}
      />
    </div>
  );
}
