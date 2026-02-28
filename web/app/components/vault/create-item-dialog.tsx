import { useState } from "react";
import {
  KeyValueEditor,
  createPair,
  type KeyValuePair,
} from "~/components/shared/key-value-editor";
import { usePutCredentials } from "~/lib/mutations";

interface CreateItemDialogProps {
  open: boolean;
  vault: string;
  onClose: () => void;
  onCreated?: (item: string) => void;
}

export function CreateItemDialog({
  open,
  vault,
  onClose,
  onCreated,
}: CreateItemDialogProps) {
  const [itemName, setItemName] = useState("");
  const [pairs, setPairs] = useState<KeyValuePair[]>([createPair()]);
  const putCreds = usePutCredentials(vault);

  if (!open) return null;

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    const name = itemName.trim();
    if (!name) return;

    const secrets: Record<string, string> = {};
    for (const p of pairs) {
      if (p.key.trim()) {
        secrets[p.key.trim()] = p.value;
      }
    }

    putCreds.mutate(
      { item: name, secrets },
      {
        onSuccess: () => {
          setItemName("");
          setPairs([createPair()]);
          onClose();
          onCreated?.(name);
        },
      },
    );
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      <div className="fixed inset-0 bg-black/50" onClick={onClose} />
      <div className="relative z-50 w-full max-w-lg rounded-lg border bg-background p-6 shadow-lg">
        <h2 className="text-lg font-semibold mb-1">New Item</h2>
        <p className="text-sm text-muted-foreground mb-4">
          Add a new item to <span className="font-medium">{vault}</span>
        </p>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-sm font-medium mb-1">Item Name</label>
            <input
              type="text"
              value={itemName}
              onChange={(e) => setItemName(e.target.value)}
              placeholder="e.g. alice@gmail.com"
              required
              className="h-9 w-full rounded-md border border-input bg-background px-3 text-sm outline-none ring-ring focus-visible:ring-2"
            />
          </div>
          <div>
            <label className="block text-sm font-medium mb-1">
              Key-Value Pairs
            </label>
            <KeyValueEditor pairs={pairs} onChange={setPairs} />
          </div>
          <div className="flex justify-end gap-2">
            <button
              type="button"
              onClick={onClose}
              className="rounded-md border px-4 py-2 text-sm font-medium hover:bg-accent"
              disabled={putCreds.isPending}
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={!itemName.trim() || putCreds.isPending}
              className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
            >
              {putCreds.isPending ? "Creating..." : "Create"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
