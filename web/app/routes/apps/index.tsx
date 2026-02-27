import { createFileRoute, Link } from "@tanstack/react-router";
import { useSuspenseQuery } from "@tanstack/react-query";
import { useState, Suspense } from "react";
import { Plus, AppWindow } from "lucide-react";
import { AppShell } from "~/components/layout/app-shell";
import { requireSettings } from "~/components/layout/auth-guard";
import { appsQuery } from "~/lib/queries";
import { SearchFilter } from "~/components/shared/search-filter";
import { EmptyState } from "~/components/shared/empty-state";
import { formatDate } from "~/lib/utils";

export const Route = createFileRoute("/apps/")({
  beforeLoad: () => requireSettings(),
  component: () => (
    <AppShell>
      <Suspense fallback={<LoadingSkeleton />}>
        <AppsPage />
      </Suspense>
    </AppShell>
  ),
});

function AppsPage() {
  const { data: apps } = useSuspenseQuery(appsQuery());
  const [search, setSearch] = useState("");

  const filtered = apps.filter(
    (a) =>
      a.vault.toLowerCase().includes(search.toLowerCase()) ||
      a.name.toLowerCase().includes(search.toLowerCase()) ||
      a.service_type.toLowerCase().includes(search.toLowerCase()),
  );

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold tracking-tight">Apps</h2>
          <p className="text-sm text-muted-foreground">
            Manage your vault-registered applications.
          </p>
        </div>
        <Link
          to="/apps/new"
          className="flex items-center gap-2 rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90"
        >
          <Plus className="h-4 w-4" />
          Create App
        </Link>
      </div>

      <SearchFilter
        value={search}
        onChange={setSearch}
        placeholder="Search apps..."
        className="max-w-sm"
      />

      {filtered.length === 0 ? (
        <EmptyState
          icon={<AppWindow className="h-12 w-12" />}
          title={search ? "No apps found" : "No apps yet"}
          description={
            search
              ? "Try a different search term."
              : "Create your first app to get started."
          }
          action={
            !search ? (
              <Link
                to="/apps/new"
                className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90"
              >
                Create App
              </Link>
            ) : undefined
          }
        />
      ) : (
        <div className="rounded-lg border">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b bg-muted/50">
                <th className="px-4 py-3 text-left font-medium">Vault</th>
                <th className="px-4 py-3 text-left font-medium">Name</th>
                <th className="px-4 py-3 text-left font-medium">Type</th>
                <th className="px-4 py-3 text-left font-medium">Created</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((app) => (
                <tr key={app.vault} className="border-b last:border-0">
                  <td className="px-4 py-3">
                    <Link
                      to="/apps/$appId"
                      params={{ appId: app.vault }}
                      className="font-medium text-primary hover:underline"
                    >
                      {app.vault}
                    </Link>
                  </td>
                  <td className="px-4 py-3">{app.name}</td>
                  <td className="px-4 py-3">
                    <span className="rounded-full bg-secondary px-2 py-0.5 text-xs">
                      {app.service_type}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-muted-foreground">
                    {formatDate(app.created_at)}
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
