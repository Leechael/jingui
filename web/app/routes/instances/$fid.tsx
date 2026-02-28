import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { useSuspenseQuery } from "@tanstack/react-query";
import { useState, Suspense } from "react";
import { ArrowLeft, Trash2, Pencil } from "lucide-react";
import { AppShell } from "~/components/layout/app-shell";
import { requireSettings } from "~/components/layout/auth-guard";
import { instanceDetailQuery } from "~/lib/queries";
import { useUpdateInstance, useDeleteInstance } from "~/lib/mutations";
import { ConfirmDeleteDialog } from "~/components/shared/confirm-delete-dialog";
import { formatDateTime } from "~/lib/utils";

export const Route = createFileRoute("/instances/$fid")({
  beforeLoad: () => requireSettings(),
  component: () => (
    <AppShell>
      <Suspense fallback={<div className="h-64 animate-pulse rounded bg-muted" />}>
        <InstanceDetailPage />
      </Suspense>
    </AppShell>
  ),
});

function InstanceDetailPage() {
  const { fid } = Route.useParams();
  const navigate = useNavigate();
  const { data: instance } = useSuspenseQuery(instanceDetailQuery(fid));
  const updateInstance = useUpdateInstance(fid);
  const deleteInstance = useDeleteInstance();
  const [editing, setEditing] = useState(false);
  const [showDelete, setShowDelete] = useState(false);
  const [editAppId, setEditAppId] = useState(instance.bound_attestation_app_id);
  const [editLabel, setEditLabel] = useState(instance.label);

  function handleUpdate(e: React.FormEvent) {
    e.preventDefault();
    updateInstance.mutate(
      { bound_attestation_app_id: editAppId, label: editLabel || undefined },
      { onSuccess: () => setEditing(false) },
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Link to="/instances" className="text-muted-foreground hover:text-foreground">
          <ArrowLeft className="h-5 w-5" />
        </Link>
        <div className="flex-1">
          <h2 className="text-2xl font-bold tracking-tight">
            {instance.label || "Instance"}
          </h2>
          <p className="font-mono text-xs text-muted-foreground">{instance.fid}</p>
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
        <form onSubmit={handleUpdate} className="max-w-lg space-y-4 rounded-lg border p-6">
          <div className="space-y-2">
            <label className="text-sm font-medium">Attestation App ID</label>
            <input
              type="text"
              value={editAppId}
              onChange={(e) => setEditAppId(e.target.value)}
              required
              className="h-9 w-full rounded-md border border-input bg-background px-3 font-mono text-sm outline-none ring-ring focus-visible:ring-2"
            />
          </div>
          <div className="space-y-2">
            <label className="text-sm font-medium">Label (optional)</label>
            <input
              type="text"
              value={editLabel}
              onChange={(e) => setEditLabel(e.target.value)}
              className="h-9 w-full rounded-md border border-input bg-background px-3 text-sm outline-none ring-ring focus-visible:ring-2"
            />
          </div>
          {updateInstance.error && (
            <p className="text-sm text-destructive">{updateInstance.error.message}</p>
          )}
          <button
            type="submit"
            disabled={updateInstance.isPending}
            className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
          >
            {updateInstance.isPending ? "Updating..." : "Update Instance"}
          </button>
        </form>
      ) : (
        <div className="rounded-lg border p-6 space-y-4">
          <div className="grid grid-cols-2 gap-4 text-sm">
            <div>
              <span className="text-muted-foreground">FID</span>
              <p className="font-mono text-xs font-medium break-all">{instance.fid}</p>
            </div>
            <div>
              <span className="text-muted-foreground">Label</span>
              <p className="font-medium">{instance.label || "-"}</p>
            </div>
            <div>
              <span className="text-muted-foreground">Public Key</span>
              <p className="font-mono text-xs break-all">{instance.public_key}</p>
            </div>
            <div>
              <span className="text-muted-foreground">Bound Vault</span>
              <p className="font-medium">{instance.bound_vault}</p>
            </div>
            <div>
              <span className="text-muted-foreground">Bound Item</span>
              <p className="font-medium">{instance.bound_item}</p>
            </div>
            <div>
              <span className="text-muted-foreground">Attestation App ID</span>
              <p className="font-mono text-xs break-all">
                {instance.bound_attestation_app_id}
              </p>
            </div>
            <div>
              <span className="text-muted-foreground">Created</span>
              <p className="font-medium">{formatDateTime(instance.created_at)}</p>
            </div>
            <div>
              <span className="text-muted-foreground">Last Used</span>
              <p className="font-medium">
                {instance.last_used_at
                  ? formatDateTime(instance.last_used_at)
                  : "Never"}
              </p>
            </div>
          </div>
        </div>
      )}

      <ConfirmDeleteDialog
        open={showDelete}
        onClose={() => setShowDelete(false)}
        onConfirm={() =>
          deleteInstance.mutate(fid, {
            onSuccess: () => navigate({ to: "/instances" }),
          })
        }
        title="Delete Instance"
        description={`This will permanently delete instance "${instance.fid.slice(0, 16)}...".`}
        isPending={deleteInstance.isPending}
      />
    </div>
  );
}
