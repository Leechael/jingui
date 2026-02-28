import { useState, useMemo } from "react";
import { Plus, KeyRound } from "lucide-react";
import { SearchFilter } from "~/components/shared/search-filter";
import type { SecretListItem } from "~/lib/types";
import { formatDateTime } from "~/lib/utils";

interface VaultItemListProps {
  secrets: SecretListItem[];
  vault: string;
  selectedItem: string | null;
  onSelectItem: (item: string) => void;
  onNewItem: () => void;
}

export function VaultItemList({
  secrets,
  vault,
  selectedItem,
  onSelectItem,
  onNewItem,
}: VaultItemListProps) {
  const [search, setSearch] = useState("");

  const items = useMemo(() => {
    const vaultItems = secrets
      .filter((s) => s.vault === vault)
      .sort((a, b) => a.item.localeCompare(b.item));
    if (!search) return vaultItems;
    const q = search.toLowerCase();
    return vaultItems.filter((s) => s.item.toLowerCase().includes(q));
  }, [secrets, vault, search]);

  return (
    <div className="flex flex-col h-full border-r">
      <div className="p-3 border-b space-y-2">
        <div className="flex items-center justify-between">
          <h3 className="text-sm font-semibold">Items</h3>
          <button
            onClick={onNewItem}
            className="flex items-center gap-1 rounded-md px-2 py-1 text-xs text-muted-foreground hover:bg-accent hover:text-accent-foreground transition-colors"
          >
            <Plus className="h-3.5 w-3.5" />
            New
          </button>
        </div>
        <SearchFilter
          value={search}
          onChange={setSearch}
          placeholder="Search items..."
        />
      </div>
      <div className="flex-1 overflow-y-auto">
        {items.map((s) => (
          <button
            key={s.item}
            onClick={() => onSelectItem(s.item)}
            className={`flex w-full items-start gap-3 px-3 py-3 text-left transition-colors border-b ${
              selectedItem === s.item
                ? "bg-accent"
                : "hover:bg-accent/50"
            }`}
          >
            <KeyRound className="h-4 w-4 mt-0.5 shrink-0 text-muted-foreground" />
            <div className="min-w-0">
              <p className="text-sm font-medium truncate">{s.item}</p>
              <p className="text-xs text-muted-foreground">
                {formatDateTime(s.updated_at)}
              </p>
            </div>
          </button>
        ))}
        {items.length === 0 && (
          <div className="px-3 py-6 text-center text-sm text-muted-foreground">
            {search ? "No matching items" : "No items yet"}
          </div>
        )}
      </div>
    </div>
  );
}
