import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { useSuspenseQuery } from "@tanstack/react-query";
import { useState, Suspense } from "react";
import { ArrowLeft, Trash2, Pencil } from "lucide-react";
import { AppShell } from "~/components/layout/app-shell";
import { requireSettings } from "~/components/layout/auth-guard";
import { appDetailQuery } from "~/lib/queries";
import { useUpdateApp, useDeleteApp } from "~/lib/mutations";
import { AppForm } from "~/components/apps/app-form";
import { CredentialsSection } from "~/components/apps/credentials-section";
import { ConfirmDeleteDialog } from "~/components/shared/confirm-delete-dialog";
import { formatDateTime } from "~/lib/utils";

export const Route = createFileRoute("/apps/$appId")({
  beforeLoad: () => requireSettings(),
  component: () => (
    <AppShell>
      <Suspense fallback={<div className="h-64 animate-pulse rounded bg-muted" />}>
        <AppDetailPage />
      </Suspense>
    </AppShell>
  ),
});

function AppDetailPage() {
  const { appId } = Route.useParams();
  const navigate = useNavigate();
  const { data: app } = useSuspenseQuery(appDetailQuery(appId));
  const updateApp = useUpdateApp(appId);
  const deleteApp = useDeleteApp();
  const [editing, setEditing] = useState(false);
  const [showDelete, setShowDelete] = useState(false);

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Link to="/apps" className="text-muted-foreground hover:text-foreground">
          <ArrowLeft className="h-5 w-5" />
        </Link>
        <div className="flex-1">
          <h2 className="text-2xl font-bold tracking-tight">{app.vault}</h2>
          <p className="text-sm text-muted-foreground">{app.name}</p>
        </div>
        <button
          onClick={() => setEditing(!editing)}
          className="flex items-center gap-2 rounded-md border px-3 py-2 text-sm hover:bg-accent"
        >
          <Pencil className="h-4 w-4" />
          {editing ? "Cancel" : "Edit"}
        </button>
        <button
          onClick={() => setShowDelete(true)}
          className="flex items-center gap-2 rounded-md border border-destructive/50 px-3 py-2 text-sm text-destructive hover:bg-destructive/10"
        >
          <Trash2 className="h-4 w-4" />
          Delete
        </button>
      </div>

      {editing ? (
        <AppForm
          defaultValues={{
            vault: app.vault,
            name: app.name,
            service_type: app.service_type,
            required_scopes: app.required_scopes,
          }}
          onSubmit={(data) =>
            updateApp.mutate(data, { onSuccess: () => setEditing(false) })
          }
          isPending={updateApp.isPending}
          error={updateApp.error?.message}
          isEdit
        />
      ) : (
        <div className="rounded-lg border p-6 space-y-4">
          <div className="grid grid-cols-2 gap-4 text-sm">
            <div>
              <span className="text-muted-foreground">Vault</span>
              <p className="font-medium">{app.vault}</p>
            </div>
            <div>
              <span className="text-muted-foreground">Name</span>
              <p className="font-medium">{app.name}</p>
            </div>
            <div>
              <span className="text-muted-foreground">Service Type</span>
              <p className="font-medium">{app.service_type}</p>
            </div>
            <div>
              <span className="text-muted-foreground">Has Credentials</span>
              <p className="font-medium">{app.has_credentials ? "Yes" : "No"}</p>
            </div>
            <div>
              <span className="text-muted-foreground">Required Scopes</span>
              <p className="font-medium">{app.required_scopes || "None"}</p>
            </div>
            <div>
              <span className="text-muted-foreground">Created</span>
              <p className="font-medium">{formatDateTime(app.created_at)}</p>
            </div>
          </div>
        </div>
      )}

      <CredentialsSection appId={appId} />

      <ConfirmDeleteDialog
        open={showDelete}
        onClose={() => setShowDelete(false)}
        onConfirm={(cascade) =>
          deleteApp.mutate(
            { appId, cascade },
            { onSuccess: () => navigate({ to: "/apps" }) },
          )
        }
        title="Delete App"
        description={`This will permanently delete the app "${app.vault}".`}
        showCascade
        cascadeLabel="Also delete dependent secrets and instances"
        isPending={deleteApp.isPending}
      />
    </div>
  );
}
