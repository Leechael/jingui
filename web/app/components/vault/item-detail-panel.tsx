import { useQuery } from "@tanstack/react-query";
import { useState, useMemo } from "react";
import { Eye, EyeOff, Copy, Pencil, Trash2, Check } from "lucide-react";
import {
  secretDetailQuery,
  secretDataQuery,
  instancesByVaultQuery,
} from "~/lib/queries";
import { useDeleteSecret, usePutCredentials } from "~/lib/mutations";
import { DebugPolicyToggle } from "~/components/secrets/debug-policy-toggle";
import { ConfirmDeleteDialog } from "~/components/shared/confirm-delete-dialog";
import {
  KeyValueEditor,
  createPair,
  type KeyValuePair,
} from "~/components/shared/key-value-editor";
import { formatDateTime } from "~/lib/utils";
import { addToast } from "~/lib/toast";

interface ItemDetailPanelProps {
  vault: string;
  item: string;
  onDeleted: () => void;
}

export function ItemDetailPanel({
  vault,
  item,
  onDeleted,
}: ItemDetailPanelProps) {
  const { data: detail, isLoading: detailLoading } = useQuery(
    secretDetailQuery(vault, item),
  );
  const { data: secretData, isLoading: dataLoading } = useQuery(
    secretDataQuery(vault, item),
  );
  const { data: instances } = useQuery(instancesByVaultQuery(vault));

  const deleteSecret = useDeleteSecret();
  const putCreds = usePutCredentials(vault);

  const [editing, setEditing] = useState(false);
  const [pairs, setPairs] = useState<KeyValuePair[]>([]);
  const [showDelete, setShowDelete] = useState(false);
  const [revealedKeys, setRevealedKeys] = useState<Set<string>>(new Set());
  const [copiedKey, setCopiedKey] = useState<string | null>(null);

  const boundInstances = useMemo(() => {
    if (!instances) return [];
    return instances.filter((i) => i.bound_item === item);
  }, [instances, item]);

  function startEditing() {
    const data = secretData?.data ?? {};
    const initial = Object.entries(data).map(([k, v]) => createPair(k, v));
    if (initial.length === 0) initial.push(createPair());
    setPairs(initial);
    setEditing(true);
  }

  function cancelEditing() {
    setEditing(false);
    setPairs([]);
  }

  function handleSave() {
    const secrets: Record<string, string> = {};
    for (const p of pairs) {
      if (p.key.trim()) {
        secrets[p.key.trim()] = p.value;
      }
    }
    putCreds.mutate(
      { item, secrets },
      {
        onSuccess: () => {
          setEditing(false);
          setPairs([]);
        },
      },
    );
  }

  function toggleReveal(key: string) {
    setRevealedKeys((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  }

  function copyValue(key: string, value: string) {
    navigator.clipboard.writeText(value);
    setCopiedKey(key);
    addToast("Copied to clipboard");
    setTimeout(() => setCopiedKey(null), 2000);
  }

  if (detailLoading) {
    return (
      <div className="flex-1 p-6">
        <div className="space-y-4">
          <div className="h-6 w-48 animate-pulse rounded bg-muted" />
          <div className="h-4 w-32 animate-pulse rounded bg-muted" />
          <div className="h-4 w-40 animate-pulse rounded bg-muted" />
        </div>
      </div>
    );
  }

  if (!detail) return null;

  return (
    <div className="flex-1 overflow-y-auto p-6">
      <div className="space-y-6">
        {/* Header */}
        <div className="flex items-start justify-between">
          <div>
            <p className="text-xs text-muted-foreground">{detail.vault}</p>
            <h3 className="text-lg font-semibold">{detail.item}</h3>
          </div>
          {!editing && (
            <button
              onClick={startEditing}
              className="flex items-center gap-2 rounded-md border px-3 py-1.5 text-sm hover:bg-accent transition-colors"
            >
              <Pencil className="h-3.5 w-3.5" />
              Edit
            </button>
          )}
        </div>

        {/* Fields - View or Edit mode */}
        {editing ? (
          <div className="space-y-4">
            <div>
              <label className="block text-sm font-medium mb-2">Fields</label>
              <KeyValueEditor pairs={pairs} onChange={setPairs} />
            </div>
            <div className="flex gap-2">
              <button
                onClick={handleSave}
                disabled={putCreds.isPending}
                className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
              >
                {putCreds.isPending ? "Saving..." : "Save"}
              </button>
              <button
                onClick={cancelEditing}
                disabled={putCreds.isPending}
                className="rounded-md border px-4 py-2 text-sm font-medium hover:bg-accent"
              >
                Cancel
              </button>
            </div>
          </div>
        ) : (
          <div className="space-y-1">
            {dataLoading ? (
              <div className="space-y-2">
                {[1, 2, 3].map((i) => (
                  <div
                    key={i}
                    className="h-12 animate-pulse rounded bg-muted"
                  />
                ))}
              </div>
            ) : secretData &&
              Object.keys(secretData.data).length > 0 ? (
              Object.entries(secretData.data).map(([key, value]) => (
                <div
                  key={key}
                  className="flex items-center justify-between rounded-md border px-4 py-3"
                >
                  <div className="min-w-0 flex-1">
                    <p className="text-xs font-medium text-muted-foreground">
                      {key}
                    </p>
                    <p className="text-sm font-mono truncate">
                      {revealedKeys.has(key)
                        ? value
                        : "\u2022\u2022\u2022\u2022\u2022\u2022\u2022\u2022"}
                    </p>
                  </div>
                  <div className="flex items-center gap-1 ml-3 shrink-0">
                    <button
                      onClick={() => toggleReveal(key)}
                      className="rounded-md p-1.5 hover:bg-accent transition-colors"
                      title={
                        revealedKeys.has(key) ? "Hide" : "Reveal"
                      }
                    >
                      {revealedKeys.has(key) ? (
                        <EyeOff className="h-4 w-4 text-muted-foreground" />
                      ) : (
                        <Eye className="h-4 w-4 text-muted-foreground" />
                      )}
                    </button>
                    <button
                      onClick={() => copyValue(key, value)}
                      className="rounded-md p-1.5 hover:bg-accent transition-colors"
                      title="Copy"
                    >
                      {copiedKey === key ? (
                        <Check className="h-4 w-4 text-green-600" />
                      ) : (
                        <Copy className="h-4 w-4 text-muted-foreground" />
                      )}
                    </button>
                  </div>
                </div>
              ))
            ) : (
              <div className="rounded-md border border-dashed px-4 py-6 text-center text-sm text-muted-foreground">
                No fields stored yet.{" "}
                <button
                  onClick={startEditing}
                  className="text-primary hover:underline"
                >
                  Add fields
                </button>
              </div>
            )}
          </div>
        )}

        {/* Metadata */}
        <div className="space-y-3 text-sm border-t pt-4">
          <div className="flex items-center justify-between">
            <span className="text-muted-foreground">Created</span>
            <span>{formatDateTime(detail.created_at)}</span>
          </div>
          <div className="flex items-center justify-between">
            <span className="text-muted-foreground">Updated</span>
            <span>{formatDateTime(detail.updated_at)}</span>
          </div>
        </div>

        {/* Debug Policy */}
        <div className="border-t pt-4">
          <DebugPolicyToggle vault={vault} item={item} />
        </div>

        {/* Bound Instances */}
        {boundInstances.length > 0 && (
          <div className="border-t pt-4">
            <p className="text-sm font-medium mb-2">Bound Instances</p>
            <div className="space-y-1">
              {boundInstances.map((inst) => (
                <div
                  key={inst.fid}
                  className="flex items-center justify-between rounded-md border px-3 py-2 text-sm"
                >
                  <span className="truncate font-mono text-xs">
                    {inst.label || inst.fid}
                  </span>
                  {inst.last_used_at && (
                    <span className="text-xs text-muted-foreground ml-2">
                      {formatDateTime(inst.last_used_at)}
                    </span>
                  )}
                </div>
              ))}
            </div>
          </div>
        )}

        {/* Delete */}
        <div className="border-t pt-4">
          <button
            onClick={() => setShowDelete(true)}
            className="flex items-center gap-2 rounded-md border border-destructive/50 px-3 py-2 text-sm text-destructive hover:bg-destructive/10"
          >
            <Trash2 className="h-4 w-4" />
            Delete Item
          </button>
        </div>
      </div>

      <ConfirmDeleteDialog
        open={showDelete}
        onClose={() => setShowDelete(false)}
        onConfirm={(cascade) =>
          deleteSecret.mutate(
            { vault, item, cascade },
            {
              onSuccess: () => {
                setShowDelete(false);
                onDeleted();
              },
            },
          )
        }
        title="Delete Item"
        description={`This will permanently delete "${vault}/${item}".`}
        showCascade
        cascadeLabel="Also delete dependent instances"
        isPending={deleteSecret.isPending}
      />
    </div>
  );
}
