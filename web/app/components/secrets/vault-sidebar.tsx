import { useMemo, useState } from "react";
import { Search } from "lucide-react";
import type { SecretListItem } from "~/lib/types";
import { cn } from "~/lib/utils";

interface VaultSidebarProps {
  secrets: SecretListItem[];
  selectedVault: string | null;
  onSelectVault: (vault: string) => void;
}

export function VaultSidebar({
  secrets,
  selectedVault,
  onSelectVault,
}: VaultSidebarProps) {
  const [search, setSearch] = useState("");

  const vaults = useMemo(() => {
    const map = new Map<string, number>();
    for (const s of secrets) {
      map.set(s.vault, (map.get(s.vault) || 0) + 1);
    }
    return Array.from(map.entries())
      .map(([name, count]) => ({ name, count }))
      .sort((a, b) => a.name.localeCompare(b.name));
  }, [secrets]);

  const filtered = vaults.filter((v) =>
    v.name.toLowerCase().includes(search.toLowerCase()),
  );

  return (
    <div className="flex h-full flex-col border-r">
      <div className="border-b p-2">
        <div className="relative">
          <Search className="absolute left-2 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" />
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search vaults..."
            className="h-8 w-full rounded-md border border-input bg-background pl-7 pr-2 text-xs outline-none ring-ring focus-visible:ring-1"
          />
        </div>
      </div>
      <div className="flex-1 overflow-y-auto">
        {filtered.map((v) => (
          <button
            key={v.name}
            onClick={() => onSelectVault(v.name)}
            className={cn(
              "flex w-full items-center justify-between px-3 py-2 text-left text-sm hover:bg-accent",
              selectedVault === v.name && "bg-accent font-medium",
            )}
          >
            <span className="truncate">{v.name}</span>
            <span className="ml-2 shrink-0 text-xs text-muted-foreground">
              {v.count}
            </span>
          </button>
        ))}
        {filtered.length === 0 && (
          <p className="p-3 text-xs text-muted-foreground">No vaults found</p>
        )}
      </div>
    </div>
  );
}
