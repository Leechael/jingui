import { createFileRoute, useNavigate, useSearch } from "@tanstack/react-router";
import { useState } from "react";
import { Inbox } from "lucide-react";
import { AppShell } from "~/components/layout/app-shell";
import { requireSettings } from "~/components/layout/auth-guard";
import { VaultItemList } from "~/components/vault/vault-item-list";
import { VaultDetailPanel } from "~/components/vault/vault-detail-panel";
import { ItemDetailPanel } from "~/components/vault/item-detail-panel";
import { CreateItemDialog } from "~/components/vault/create-item-dialog";
import { EmptyState } from "~/components/shared/empty-state";

type VaultSearch = {
  vault?: string;
  item?: string;
};

export const Route = createFileRoute("/")({
  beforeLoad: () => requireSettings(),
  validateSearch: (search: Record<string, unknown>): VaultSearch => ({
    vault: typeof search.vault === "string" ? search.vault : undefined,
    item: typeof search.item === "string" ? search.item : undefined,
  }),
  component: () => (
    <AppShell fullWidth>
      <VaultBrowser />
    </AppShell>
  ),
});

function VaultBrowser() {
  const search = useSearch({ from: "/" });
  const navigate = useNavigate({ from: "/" });

  const selectedVault = search.vault ?? null;
  const selectedItem = search.item ?? null;

  const [showCreateItem, setShowCreateItem] = useState(false);

  function selectItem(item: string) {
    navigate({ search: { vault: selectedVault!, item } });
  }

  function onItemDeleted() {
    navigate({ search: { vault: selectedVault!, item: undefined } });
  }

  function onVaultDeleted() {
    navigate({ search: {} });
  }

  if (!selectedVault) {
    return (
      <div className="flex h-full items-center justify-center">
        <EmptyState
          icon={<Inbox className="h-12 w-12" />}
          title="Select a vault"
          description="Choose a vault from the sidebar to browse its items."
        />
      </div>
    );
  }

  return (
    <div className="flex h-full">
      <div className="w-[250px] shrink-0">
        <VaultItemList
          vault={selectedVault}
          selectedItem={selectedItem}
          onSelectItem={selectItem}
          onNewItem={() => setShowCreateItem(true)}
        />
      </div>

      {selectedItem ? (
        <ItemDetailPanel
          key={`${selectedVault}/${selectedItem}`}
          vault={selectedVault}
          item={selectedItem}
          onDeleted={onItemDeleted}
        />
      ) : (
        <VaultDetailPanel
          vault={selectedVault}
          onDeleted={onVaultDeleted}
        />
      )}

      <CreateItemDialog
        open={showCreateItem}
        vault={selectedVault}
        onClose={() => setShowCreateItem(false)}
        onCreated={(item) => {
          setShowCreateItem(false);
          selectItem(item);
        }}
      />
    </div>
  );
}
