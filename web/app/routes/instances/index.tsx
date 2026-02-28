import { createFileRoute, Link } from "@tanstack/react-router";
import { useSuspenseQuery } from "@tanstack/react-query";
import { useState, Suspense } from "react";
import { Plus, Server } from "lucide-react";
import { AppShell } from "~/components/layout/app-shell";
import { requireSettings } from "~/components/layout/auth-guard";
import { instancesQuery } from "~/lib/queries";
import { SearchFilter } from "~/components/shared/search-filter";
import { EmptyState } from "~/components/shared/empty-state";
import { formatDate, truncate } from "~/lib/utils";

export const Route = createFileRoute("/instances/")({
  beforeLoad: () => requireSettings(),
  component: () => (
    <AppShell>
      <Suspense fallback={<LoadingSkeleton />}>
        <InstancesPage />
      </Suspense>
    </AppShell>
  ),
});

function InstancesPage() {
  const { data: instances } = useSuspenseQuery(instancesQuery());
  const [search, setSearch] = useState("");

  const filtered = instances.filter((i) => {
    const q = search.toLowerCase();
    return (
      i.fid.toLowerCase().includes(q) ||
      i.dstack_app_id.toLowerCase().includes(q) ||
      (i.label ?? "").toLowerCase().includes(q)
    );
  });

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold tracking-tight">Instances</h2>
          <p className="text-sm text-muted-foreground">
            Manage registered TEE instances.
          </p>
        </div>
        <Link
          to="/instances/new"
          className="flex items-center gap-2 rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90"
        >
          <Plus className="h-4 w-4" />
          Register Instance
        </Link>
      </div>

      <SearchFilter
        value={search}
        onChange={setSearch}
        placeholder="Search instances..."
        className="max-w-sm"
      />

      {filtered.length === 0 ? (
        <EmptyState
          icon={<Server className="h-12 w-12" />}
          title={search ? "No instances found" : "No instances yet"}
          description={
            search
              ? "Try a different search term."
              : "Register your first TEE instance to get started."
          }
          action={
            !search ? (
              <Link
                to="/instances/new"
                className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90"
              >
                Register Instance
              </Link>
            ) : undefined
          }
        />
      ) : (
        <div className="rounded-lg border">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b bg-muted/50">
                <th className="px-4 py-3 text-left font-medium">
                  Dstack App ID
                </th>
                <th className="px-4 py-3 text-left font-medium">Label</th>
                <th className="px-4 py-3 text-left font-medium">Created</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((inst) => (
                <tr key={inst.fid} className="border-b last:border-0">
                  <td className="px-4 py-3">
                    <Link
                      to="/instances/$fid"
                      params={{ fid: inst.fid }}
                      className="font-mono text-xs font-medium text-primary hover:underline"
                    >
                      {truncate(inst.dstack_app_id, 16)}
                    </Link>
                  </td>
                  <td className="px-4 py-3">{inst.label || "-"}</td>
                  <td className="px-4 py-3 text-muted-foreground">
                    {formatDate(inst.created_at)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

function LoadingSkeleton() {
  return (
    <div className="space-y-4">
      <div className="h-8 w-48 animate-pulse rounded bg-muted" />
      <div className="h-9 w-64 animate-pulse rounded bg-muted" />
      <div className="space-y-2">
        {[1, 2, 3].map((i) => (
          <div key={i} className="h-12 animate-pulse rounded bg-muted" />
        ))}
      </div>
    </div>
  );
}
