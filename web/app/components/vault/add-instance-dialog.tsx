import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { instancesQuery } from "~/lib/queries";
import {
  useGrantVaultAccess,
  useRegisterInstance,
} from "~/lib/mutations";
import { truncate } from "~/lib/utils";

interface AddInstanceDialogProps {
  open: boolean;
  onClose: () => void;
  vaultId: string;
  linkedFids: string[];
}

type Tab = "existing" | "register";

export function AddInstanceDialog({
  open,
  onClose,
  vaultId,
  linkedFids,
}: AddInstanceDialogProps) {
  const { data: allInstances, isLoading } = useQuery(instancesQuery());
  const grantAccess = useGrantVaultAccess(vaultId);
  const registerInstance = useRegisterInstance();

  const [tab, setTab] = useState<Tab>("existing");
  const [publicKey, setPublicKey] = useState("");
  const [dstackAppId, setDstackAppId] = useState("");
  const [label, setLabel] = useState("");

  if (!open) return null;

  const unlinked =
    allInstances?.filter((inst) => !linkedFids.includes(inst.fid)) ?? [];

  function resetForm() {
    setPublicKey("");
    setDstackAppId("");
    setLabel("");
  }

  function handleRegisterAndLink(e: React.FormEvent) {
    e.preventDefault();
    registerInstance.mutate(
      {
        public_key: publicKey,
        dstack_app_id: dstackAppId,
        label: label || undefined,
      },
      {
        onSuccess: (data) => {
          grantAccess.mutate(data.fid);
          resetForm();
          setTab("existing");
        },
      },
    );
  }

  const isPending = registerInstance.isPending || grantAccess.isPending;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      <div className="fixed inset-0 bg-black/50" onClick={onClose} />
      <div className="relative z-50 w-full max-w-md rounded-lg border bg-background p-6 shadow-lg">
        <h2 className="text-lg font-semibold mb-4">Add Instance</h2>

        <div className="flex gap-1 rounded-md bg-muted p-1 mb-4">
          <button
            onClick={() => setTab("existing")}
            className={`flex-1 rounded-sm px-3 py-1.5 text-sm font-medium transition-colors ${
              tab === "existing"
                ? "bg-background shadow-sm"
                : "text-muted-foreground hover:text-foreground"
            }`}
          >
            Link Existing
          </button>
          <button
            onClick={() => setTab("register")}
            className={`flex-1 rounded-sm px-3 py-1.5 text-sm font-medium transition-colors ${
              tab === "register"
                ? "bg-background shadow-sm"
                : "text-muted-foreground hover:text-foreground"
            }`}
          >
            Register New
          </button>
        </div>

        {tab === "existing" ? (
          <div className="space-y-2 max-h-64 overflow-y-auto">
            {unlinked.length > 0 ? (
              unlinked.map((inst) => (
                <div
                  key={inst.fid}
                  className="flex items-center justify-between gap-3 rounded-md border p-3"
                >
                  <div className="min-w-0">
                    <p className="text-sm font-medium">
                      {inst.label || truncate(inst.fid, 20)}
                    </p>
                    <code className="text-xs text-muted-foreground break-all">
                      {inst.dstack_app_id}
                    </code>
                  </div>
                  <button
                    onClick={() => grantAccess.mutate(inst.fid)}
                    disabled={grantAccess.isPending}
                    className="shrink-0 rounded-md bg-primary px-3 py-1 text-xs font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
                  >
                    Grant Access
                  </button>
                </div>
              ))
            ) : isLoading ? (
              <p className="text-sm text-muted-foreground">
                Loading instances...
              </p>
            ) : (
              <p className="text-sm text-muted-foreground">
                All instances are already linked.
              </p>
            )}
          </div>
        ) : (
          <form onSubmit={handleRegisterAndLink} className="space-y-4">
            <div className="rounded-md bg-muted/50 p-3 text-xs text-muted-foreground">
              Run on the TEE instance to get these values:
              <pre className="mt-1.5 rounded bg-muted px-2 py-1.5 font-mono">
                jingui status --server $SERVER
              </pre>
            </div>

            <div className="space-y-2">
              <label className="text-sm font-medium">Public Key</label>
              <input
                type="text"
                value={publicKey}
                onChange={(e) => setPublicKey(e.target.value)}
                required
                placeholder="64 hex characters (32 bytes)"
                pattern="[0-9a-fA-F]{64}"
                className="h-9 w-full rounded-md border border-input bg-background px-3 font-mono text-sm outline-none ring-ring focus-visible:ring-2"
              />
              <p className="text-xs text-muted-foreground">
                <code>public_key</code> from status output
              </p>
            </div>

            <div className="space-y-2">
              <label className="text-sm font-medium">Dstack App ID</label>
              <input
                type="text"
                value={dstackAppId}
                onChange={(e) => setDstackAppId(e.target.value)}
                required
                placeholder="Hex-encoded RA-TLS identity"
                className="h-9 w-full rounded-md border border-input bg-background px-3 font-mono text-sm outline-none ring-ring focus-visible:ring-2"
              />
              <p className="text-xs text-muted-foreground">
                <code>dstack_app_id</code> from status output
              </p>
            </div>

            <div className="space-y-2">
              <label className="text-sm font-medium">Label (optional)</label>
              <input
                type="text"
                value={label}
                onChange={(e) => setLabel(e.target.value)}
                placeholder="Production worker #1"
                className="h-9 w-full rounded-md border border-input bg-background px-3 text-sm outline-none ring-ring focus-visible:ring-2"
              />
            </div>

            <button
              type="submit"
              disabled={isPending}
              className="w-full rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
            >
              {isPending ? "Registering..." : "Register & Link"}
            </button>
          </form>
        )}

        <div className="flex justify-end mt-4">
          <button
            type="button"
            onClick={onClose}
            className="rounded-md border px-4 py-2 text-sm font-medium hover:bg-accent"
          >
            Close
          </button>
        </div>
      </div>
    </div>
  );
}
