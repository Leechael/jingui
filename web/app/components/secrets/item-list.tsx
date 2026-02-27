import { useMemo, useState } from "react";
import { Search } from "lucide-react";
import type { SecretListItem } from "~/lib/types";
import { cn, formatDate } from "~/lib/utils";

interface ItemListProps {
  secrets: SecretListItem[];
  vault: string;
  selectedItem: string | null;
  onSelectItem: (item: string) => void;
}

export function ItemList({
  secrets,
  vault,
  selectedItem,
  onSelectItem,
}: ItemListProps) {
  const [search, setSearch] = useState("");

  const items = useMemo(
    () =>
      secrets
        .filter((s) => s.vault === vault)
        .sort((a, b) => a.item.localeCompare(b.item)),
    [secrets, vault],
  );

  const filtered = items.filter((i) =>
    i.item.toLowerCase().includes(search.toLowerCase()),
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
            placeholder="Search items..."
            className="h-8 w-full rounded-md border border-input bg-background pl-7 pr-2 text-xs outline-none ring-ring focus-visible:ring-1"
          />
        </div>
      </div>
      <div className="flex-1 overflow-y-auto">
        {filtered.map((s) => (
          <button
            key={s.item}
            onClick={() => onSelectItem(s.item)}
            className={cn(
              "flex w-full flex-col px-3 py-2 text-left hover:bg-accent",
              selectedItem === s.item && "bg-accent",
            )}
          >
            <span
              className={cn(
                "truncate text-sm",
                selectedItem === s.item && "font-medium",
              )}
            >
              {s.item}
            </span>
            <span className="text-xs text-muted-foreground">
              {formatDate(s.updated_at)}
            </span>
          </button>
        ))}
        {filtered.length === 0 && (
          <p className="p-3 text-xs text-muted-foreground">No items found</p>
        )}
      </div>
    </div>
  );
}
