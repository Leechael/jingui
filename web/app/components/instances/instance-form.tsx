import { useState } from "react";
import type { InstanceRequest } from "~/lib/types";

interface InstanceFormProps {
  onSubmit: (data: InstanceRequest) => void;
  isPending: boolean;
  error?: string;
}

export function InstanceForm({ onSubmit, isPending, error }: InstanceFormProps) {
  const [publicKey, setPublicKey] = useState("");
  const [dstackAppId, setDstackAppId] = useState("");
  const [label, setLabel] = useState("");

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    onSubmit({
      public_key: publicKey,
      dstack_app_id: dstackAppId,
      label: label || undefined,
    });
  }

  return (
    <form onSubmit={handleSubmit} className="max-w-lg space-y-4 rounded-lg border p-6">
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
          X25519 public key as 64 hex characters
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

      {error && <p className="text-sm text-destructive">{error}</p>}

      <button
        type="submit"
        disabled={isPending}
        className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
      >
        {isPending ? "Registering..." : "Register Instance"}
      </button>
    </form>
  );
}
