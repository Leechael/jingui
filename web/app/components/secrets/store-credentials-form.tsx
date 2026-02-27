import { useState } from "react";
import { useSuspenseQuery } from "@tanstack/react-query";
import { KeyRound, X } from "lucide-react";
import { appsQuery } from "~/lib/queries";
import { usePutCredentials } from "~/lib/mutations";
import {
  KeyValueEditor,
  type KeyValuePair,
} from "~/components/shared/key-value-editor";

interface StoreCredentialsFormProps {
  open: boolean;
  onClose: () => void;
  defaultItem?: string;
}

export function StoreCredentialsForm({
  open,
  onClose,
  defaultItem,
}: StoreCredentialsFormProps) {
  const { data: apps } = useSuspenseQuery(appsQuery());
  const [selectedApp, setSelectedApp] = useState("");
  const [item, setItem] = useState(defaultItem ?? "");
  const [pairs, setPairs] = useState<KeyValuePair[]>([
    { key: "", value: "" },
  ]);

  const putCreds = usePutCredentials(selectedApp);

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    const secrets: Record<string, string> = {};
    for (const p of pairs) {
      if (p.key.trim()) secrets[p.key.trim()] = p.value;
    }
    if (!selectedApp || !item.trim() || Object.keys(secrets).length === 0)
      return;
    putCreds.mutate(
      { item: item.trim(), secrets },
      {
        onSuccess: () => {
          onClose();
          setSelectedApp("");
          setItem(defaultItem ?? "");
          setPairs([{ key: "", value: "" }]);
        },
      },
    );
  }

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      <div className="fixed inset-0 bg-black/50" onClick={onClose} />
      <div className="relative z-50 w-full max-w-md rounded-lg border bg-background p-6 shadow-lg">
        <div className="mb-4 flex items-center justify-between">
          <h3 className="text-lg font-semibold flex items-center gap-2">
            <KeyRound className="h-5 w-5" />
            Store Credentials
          </h3>
          <button
            onClick={onClose}
            className="rounded-md p-1 hover:bg-accent"
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-2">
            <label className="text-sm font-medium">App</label>
            <select
              value={selectedApp}
              onChange={(e) => setSelectedApp(e.target.value)}
              required
              className="h-9 w-full rounded-md border border-input bg-background px-3 text-sm outline-none ring-ring focus-visible:ring-2"
            >
              <option value="">Select an app...</option>
              {apps.map((a) => (
                <option key={a.vault} value={a.vault}>
                  {a.vault} — {a.name}
                </option>
              ))}
            </select>
          </div>

          <div className="space-y-2">
            <label className="text-sm font-medium">Item Name</label>
            <input
              type="text"
              value={item}
              onChange={(e) => setItem(e.target.value)}
              required
              placeholder="alice@gmail.com"
              className="h-9 w-full rounded-md border border-input bg-background px-3 text-sm outline-none ring-ring focus-visible:ring-2"
            />
          </div>

          <div className="space-y-2">
            <label className="text-sm font-medium">Secrets</label>
            <KeyValueEditor pairs={pairs} onChange={setPairs} />
          </div>

          {putCreds.error && (
            <p className="text-sm text-destructive">
              {putCreds.error.message}
            </p>
          )}

          <div className="flex justify-end gap-2">
            <button
              type="button"
              onClick={onClose}
              className="rounded-md border px-4 py-2 text-sm font-medium hover:bg-accent"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={putCreds.isPending}
              className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
            >
              {putCreds.isPending ? "Storing..." : "Store"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
