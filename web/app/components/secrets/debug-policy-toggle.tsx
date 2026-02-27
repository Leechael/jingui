import { useQuery } from "@tanstack/react-query";
import { useUpdateDebugPolicy } from "~/lib/mutations";
import { debugPolicyQuery } from "~/lib/queries";

interface DebugPolicyToggleProps {
  vault: string;
  item: string;
}

export function DebugPolicyToggle({ vault, item }: DebugPolicyToggleProps) {
  const { data: policy, isLoading } = useQuery(debugPolicyQuery(vault, item));
  const update = useUpdateDebugPolicy(vault, item);

  function handleToggle() {
    if (!policy) return;
    update.mutate({ allow_read_debug: !policy.allow_read_debug });
  }

  return (
    <div className="flex items-center justify-between">
      <div>
        <p className="text-sm font-medium">Debug Read</p>
        <p className="text-xs text-muted-foreground">
          Allow debug read access for this secret
        </p>
      </div>
      <button
        onClick={handleToggle}
        disabled={isLoading || update.isPending}
        className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors disabled:opacity-50 ${
          policy?.allow_read_debug ? "bg-primary" : "bg-muted"
        }`}
      >
        <span
          className={`inline-block h-4 w-4 rounded-full bg-white transition-transform ${
            policy?.allow_read_debug ? "translate-x-6" : "translate-x-1"
          }`}
        />
      </button>
    </div>
  );
}
