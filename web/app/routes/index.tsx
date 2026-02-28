import { createFileRoute, useNavigate, useSearch } from "@tanstack/react-router";
import { useSuspenseQuery } from "@tanstack/react-query";
import { Suspense, useState } from "react";
import { Vault, Inbox } from "lucide-react";
import { AppShell } from "~/components/layout/app-shell";
import { requireSettings } from "~/components/layout/auth-guard";
import { secretsQuery } from "~/lib/queries";
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
      <Suspense fallback={<LoadingSkeleton />}>
        <VaultBrowser />
      </Suspense>
    </AppShell>
  ),
});

function VaultBrowser() {
  const { data: secrets } = useSuspenseQuery(secretsQuery());
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

  // No vault selected — show welcome
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
      {/* Item list pane */}
      <div className="w-[250px] shrink-0">
        <VaultItemList
          secrets={secrets}
          vault={selectedVault}
          selectedItem={selectedItem}
          onSelectItem={selectItem}
          onNewItem={() => setShowCreateItem(true)}
        />
      </div>

      {/* Detail pane */}
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

function LoadingSkeleton() {
  return (
    <div className="flex h-full">
      <div className="w-[250px] shrink-0 border-r p-3 space-y-2">
        {[1, 2, 3, 4].map((i) => (
          <div key={i} className="h-10 animate-pulse rounded bg-muted" />
        ))}
      </div>
      <div className="flex flex-1 items-center justify-center text-sm text-muted-foreground">
        Loading...
      </div>
    </div>
  );
}
