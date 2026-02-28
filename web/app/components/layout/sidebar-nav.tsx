import { Link, useRouterState, useNavigate } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { useState, useMemo } from "react";
import { Inbox, Server, Settings, Plus, Vault } from "lucide-react";
import { cn } from "~/lib/utils";
import { getSettings } from "~/lib/settings";
import { vaultsQuery, vaultItemsQuery } from "~/lib/queries";
import { SearchFilter } from "~/components/shared/search-filter";
import { CreateVaultDialog } from "~/components/vault/create-vault-dialog";

const topNav = [
  { to: "/", label: "All Items", icon: Inbox, exact: true },
  { to: "/instances", label: "Instances", icon: Server },
  { to: "/settings", label: "Settings", icon: Settings },
] as const;

export function SidebarNav() {
  const routerState = useRouterState();
  const pathname = routerState.location.pathname;
  const searchParams = routerState.location.search as Record<string, string>;
  const selectedVault = searchParams.vault ?? null;

  const configured = !!getSettings();

  return (
    <nav className="flex flex-col h-full">
      <div className="p-3">
        <div className="mb-4 px-3 py-2">
          <h1 className="text-lg font-bold tracking-tight">Jingui</h1>
          <p className="text-xs text-muted-foreground">Admin Panel</p>
        </div>
        <div className="flex flex-col gap-1">
          {topNav.map(({ to, label, icon: Icon, ...rest }) => {
            const exact = "exact" in rest && rest.exact;
            const isActive = exact
              ? pathname === to
              : pathname === to || pathname.startsWith(to + "/");
            return (
              <Link
                key={to}
                to={to}
                className={cn(
                  "flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors",
                  isActive
                    ? "bg-accent text-accent-foreground"
                    : "text-muted-foreground hover:bg-accent hover:text-accent-foreground",
                )}
              >
                <Icon className="h-4 w-4" />
                {label}
              </Link>
            );
          })}
        </div>
      </div>

      {configured && <VaultList selectedVault={selectedVault} />}
    </nav>
  );
}

function VaultList({ selectedVault }: { selectedVault: string | null }) {
  const { data: vaults } = useQuery(vaultsQuery());
  const [search, setSearch] = useState("");
  const [showCreate, setShowCreate] = useState(false);
  const navigate = useNavigate();

  const filtered = useMemo(() => {
    if (!vaults) return [];
    if (!search) return vaults;
    const q = search.toLowerCase();
    return vaults.filter(
      (v) =>
        v.id.toLowerCase().includes(q) ||
        v.name.toLowerCase().includes(q),
    );
  }, [vaults, search]);

  return (
    <>
      <div className="flex-1 flex flex-col min-h-0 border-t">
        <div className="px-3 pt-3 pb-1">
          <p className="px-3 text-xs font-semibold text-muted-foreground uppercase tracking-wider">
            Vaults
          </p>
        </div>
        <div className="px-3 pb-2">
          <SearchFilter
            value={search}
            onChange={setSearch}
            placeholder="Search vaults..."
          />
        </div>
        <div className="flex-1 overflow-y-auto px-3 space-y-0.5">
          {filtered.map((vault) => {
            const isActive = selectedVault === vault.id;
            return (
              <VaultEntry
                key={vault.id}
                vaultId={vault.id}
                isActive={isActive}
                onClick={() =>
                  navigate({ to: "/", search: { vault: vault.id } })
                }
              />
            );
          })}
          {filtered.length === 0 && vaults && vaults.length > 0 && (
            <p className="px-3 py-2 text-xs text-muted-foreground">
              No matching vaults
            </p>
          )}
          {vaults && vaults.length === 0 && (
            <p className="px-3 py-2 text-xs text-muted-foreground">
              No vaults yet
            </p>
          )}
        </div>
        <div className="p-3 border-t">
          <button
            onClick={() => setShowCreate(true)}
            className="flex w-full items-center justify-center gap-2 rounded-md border border-dashed px-3 py-2 text-sm text-muted-foreground hover:bg-accent hover:text-accent-foreground transition-colors"
          >
            <Plus className="h-4 w-4" />
            New Vault
          </button>
        </div>
      </div>

      <CreateVaultDialog
        open={showCreate}
        onClose={() => setShowCreate(false)}
        onCreated={(vault) =>
          navigate({ to: "/", search: { vault } })
        }
      />
    </>
  );
}

function VaultEntry({
  vaultId,
  isActive,
  onClick,
}: {
  vaultId: string;
  isActive: boolean;
  onClick: () => void;
}) {
  const { data: items } = useQuery(vaultItemsQuery(vaultId));
  const count = items?.length ?? 0;

  return (
    <button
      onClick={onClick}
      className={cn(
        "flex w-full items-center justify-between rounded-md px-3 py-2 text-sm transition-colors text-left",
        isActive
          ? "bg-accent text-accent-foreground font-medium"
          : "text-muted-foreground hover:bg-accent hover:text-accent-foreground",
      )}
    >
      <span className="flex items-center gap-2 truncate">
        <Vault className="h-4 w-4 shrink-0" />
        <span className="truncate">{vaultId}</span>
      </span>
      {count > 0 && (
        <span className="ml-2 text-xs text-muted-foreground">
          {count}
        </span>
      )}
    </button>
  );
}
