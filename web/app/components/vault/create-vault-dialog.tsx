import { useState } from "react";
import { useCreateVault } from "~/lib/mutations";

interface CreateVaultDialogProps {
  open: boolean;
  onClose: () => void;
  onCreated?: (vault: string) => void;
}

export function CreateVaultDialog({
  open,
  onClose,
  onCreated,
}: CreateVaultDialogProps) {
  const [vaultId, setVaultId] = useState("");
  const [name, setName] = useState("");
  const createVault = useCreateVault();

  if (!open) return null;

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    const id = vaultId.trim();
    const n = name.trim() || id;
    createVault.mutate(
      { id, name: n },
      {
        onSuccess: () => {
          setVaultId("");
          setName("");
          onClose();
          onCreated?.(id);
        },
      },
    );
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      <div className="fixed inset-0 bg-black/50" onClick={onClose} />
      <div className="relative z-50 w-full max-w-md rounded-lg border bg-background p-6 shadow-lg">
        <h2 className="text-lg font-semibold mb-4">New Vault</h2>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-sm font-medium mb-1">Vault ID</label>
            <input
              type="text"
              value={vaultId}
              onChange={(e) => setVaultId(e.target.value)}
              placeholder="e.g. gmail-vault"
              required
              className="h-9 w-full rounded-md border border-input bg-background px-3 text-sm outline-none ring-ring focus-visible:ring-2"
            />
          </div>
          <div>
            <label className="block text-sm font-medium mb-1">
              Display Name
            </label>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g. Gmail Vault"
              className="h-9 w-full rounded-md border border-input bg-background px-3 text-sm outline-none ring-ring focus-visible:ring-2"
            />
            <p className="text-xs text-muted-foreground mt-1">
              Optional. Defaults to vault ID.
            </p>
          </div>
          <div className="flex justify-end gap-2">
            <button
              type="button"
              onClick={onClose}
              className="rounded-md border px-4 py-2 text-sm font-medium hover:bg-accent"
              disabled={createVault.isPending}
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={!vaultId.trim() || createVault.isPending}
              className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
            >
              {createVault.isPending ? "Creating..." : "Create"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
