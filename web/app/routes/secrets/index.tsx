import { createFileRoute, useNavigate, useSearch } from "@tanstack/react-router";
import { useSuspenseQuery } from "@tanstack/react-query";
import { Suspense } from "react";
import { KeyRound } from "lucide-react";
import { AppShell } from "~/components/layout/app-shell";
import { requireSettings } from "~/components/layout/auth-guard";
import { secretsQuery } from "~/lib/queries";
import { VaultSidebar } from "~/components/secrets/vault-sidebar";
import { ItemList } from "~/components/secrets/item-list";
import { ItemDetail } from "~/components/secrets/item-detail";
import { EmptyState } from "~/components/shared/empty-state";

type SecretsSearch = {
  vault?: string;
  item?: string;
};

export const Route = createFileRoute("/secrets/")({
  beforeLoad: () => requireSettings(),
  validateSearch: (search: Record<string, unknown>): SecretsSearch => ({
    vault: typeof search.vault === "string" ? search.vault : undefined,
    item: typeof search.item === "string" ? search.item : undefined,
  }),
  component: () => (
    <AppShell>
      <Suspense fallback={<LoadingSkeleton />}>
        <SecretsPage />
      </Suspense>
    </AppShell>
  ),
});

function SecretsPage() {
  const { data: secrets } = useSuspenseQuery(secretsQuery());
  const search = useSearch({ from: "/secrets/" });
  const navigate = useNavigate({ from: "/secrets/" });

  const selectedVault = search.vault ?? null;
  const selectedItem = search.item ?? null;

  function selectVault(vault: string) {
    navigate({
      search: { vault, item: undefined },
    });
  }

  function selectItem(item: string) {
    navigate({
      search: { vault: selectedVault!, item },
    });
  }

  function onItemDeleted() {
    navigate({
      search: { vault: selectedVault!, item: undefined },
    });
  }

  if (secrets.length === 0) {
    return (
      <EmptyState
        icon={<KeyRound className="h-12 w-12" />}
        title="No secrets yet"
        description="Store credentials via the Apps page to create secrets."
      />
    );
  }

  return (
    <div className="-mx-6 -mt-6 flex h-[calc(100vh-0px)]">
      <div className="w-[200px] shrink-0">
        <VaultSidebar
          secrets={secrets}
          selectedVault={selectedVault}
          onSelectVault={selectVault}
        />
      </div>

      {selectedVault ? (
        <>
          <div className="w-[250px] shrink-0">
            <ItemList
              secrets={secrets}
              vault={selectedVault}
              selectedItem={selectedItem}
              onSelectItem={selectItem}
            />
          </div>

          {selectedItem ? (
            <ItemDetail
              vault={selectedVault}
              item={selectedItem}
              onDeleted={onItemDeleted}
            />
          ) : (
            <div className="flex flex-1 items-center justify-center text-sm text-muted-foreground">
              Select an item to view details
            </div>
          )}
        </>
      ) : (
        <div className="flex flex-1 items-center justify-center text-sm text-muted-foreground">
          Select a vault to browse items
        </div>
      )}
    </div>
  );
}

function LoadingSkeleton() {
  return (
    <div className="-mx-6 -mt-6 flex h-[calc(100vh-0px)]">
      <div className="w-[200px] shrink-0 border-r p-3 space-y-2">
        {[1, 2, 3, 4].map((i) => (
          <div key={i} className="h-8 animate-pulse rounded bg-muted" />
        ))}
      </div>
      <div className="flex flex-1 items-center justify-center text-sm text-muted-foreground">
        Loading...
      </div>
    </div>
  );
}
