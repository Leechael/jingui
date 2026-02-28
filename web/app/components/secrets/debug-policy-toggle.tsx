import { useQuery } from "@tanstack/react-query";
import { useUpdateDebugPolicy } from "~/lib/mutations";
import { debugPolicyQuery } from "~/lib/queries";

interface DebugPolicyToggleProps {
  vaultId: string;
  fid: string;
}

export function DebugPolicyToggle({ vaultId, fid }: DebugPolicyToggleProps) {
  const { data: policy, isLoading } = useQuery(debugPolicyQuery(vaultId, fid));
  const update = useUpdateDebugPolicy(vaultId, fid);

  function handleToggle() {
    if (!policy) return;
    update.mutate({ allow_read: !policy.allow_read });
  }

  const isDefault = policy?.source === "default";

  return (
    <div className="flex items-center justify-between">
      <div>
        <p className="text-sm font-medium">Debug Read</p>
        <p className="text-xs text-muted-foreground">
          Allow plaintext read for this vault
          {isDefault && (
            <span className="ml-1 text-muted-foreground/60">(default)</span>
          )}
        </p>
      </div>
      <button
        onClick={handleToggle}
        disabled={isLoading || update.isPending}
        className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors disabled:opacity-50 ${
          policy?.allow_read ? "bg-primary" : "bg-muted"
        }`}
      >
        <span
          className={`inline-block h-4 w-4 rounded-full bg-white transition-transform ${
            policy?.allow_read ? "translate-x-6" : "translate-x-1"
          }`}
        />
      </button>
    </div>
  );
}
