import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { Pencil, Trash2, Check, Link2 } from "lucide-react";
import { vaultItemDetailQuery } from "~/lib/queries";
import { usePutItem, useDeleteItem } from "~/lib/mutations";
import { ConfirmDeleteDialog } from "~/components/shared/confirm-delete-dialog";
import {
  KeyValueEditor,
  createPair,
  type KeyValuePair,
} from "~/components/shared/key-value-editor";
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
  const { data: itemData, isLoading } = useQuery(
    vaultItemDetailQuery(vault, item),
  );
  const putItem = usePutItem(vault);
  const deleteItem = useDeleteItem(vault);

  const [editing, setEditing] = useState(false);
  const [pairs, setPairs] = useState<KeyValuePair[]>([]);
  const [originalKeys, setOriginalKeys] = useState<string[]>([]);
  const [showDelete, setShowDelete] = useState(false);
  const [copiedRef, setCopiedRef] = useState<string | null>(null);

  function startEditing() {
    const keys = itemData?.keys ?? [];
    setOriginalKeys(keys);
    const initial = keys.map((k) => createPair(k, ""));
    if (initial.length === 0) initial.push(createPair());
    setPairs(initial);
    setEditing(true);
  }

  function cancelEditing() {
    setEditing(false);
    setPairs([]);
    setOriginalKeys([]);
  }

  function handleSave() {
    const fields: Record<string, string> = {};
    const currentKeys = new Set<string>();
    for (const p of pairs) {
      const key = p.key.trim();
      if (key) {
        currentKeys.add(key);
        if (p.value) {
          fields[key] = p.value;
        }
      }
    }
    const deleteKeys = originalKeys.filter((k) => !currentKeys.has(k));

    putItem.mutate(
      { section: item, fields, delete: deleteKeys },
      {
        onSuccess: () => {
          setEditing(false);
          setPairs([]);
          setOriginalKeys([]);
        },
      },
    );
  }

  function copyRef(key: string) {
    navigator.clipboard.writeText(`jingui://${vault}/${item}/${key}`);
    setCopiedRef(key);
    addToast("Reference URI copied");
    setTimeout(() => setCopiedRef(null), 2000);
  }

  if (isLoading) {
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

  if (!itemData) return null;

  const keys = itemData.keys ?? [];

  return (
    <div className="flex-1 overflow-y-auto p-6">
      <div className="space-y-6">
        <div className="flex items-start justify-between">
          <div>
            <p className="text-xs text-muted-foreground">{vault}</p>
            <h3 className="text-lg font-semibold">{item}</h3>
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

        {editing ? (
          <div className="space-y-4">
            <div>
              <label className="block text-sm font-medium mb-2">Fields</label>
              <p className="text-xs text-muted-foreground mb-2">
                Leave value blank to keep current value unchanged.
              </p>
              <KeyValueEditor pairs={pairs} onChange={setPairs} />
            </div>
            <div className="flex gap-2">
              <button
                onClick={handleSave}
                disabled={putItem.isPending}
                className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
              >
                {putItem.isPending ? "Saving..." : "Save"}
              </button>
              <button
                onClick={cancelEditing}
                disabled={putItem.isPending}
                className="rounded-md border px-4 py-2 text-sm font-medium hover:bg-accent"
              >
                Cancel
              </button>
            </div>
          </div>
        ) : (
          <div className="space-y-1">
            {keys.length > 0 ? (
              keys.map((key) => (
                <div
                  key={key}
                  className="flex items-center justify-between rounded-md border px-4 py-3"
                >
                  <div className="min-w-0 flex-1">
                    <p className="text-xs font-medium text-muted-foreground">
                      {key}
                    </p>
                    <p className="text-sm font-mono truncate text-muted-foreground">
                      ••••••••
                    </p>
                  </div>
                  <div className="flex items-center gap-1 ml-3 shrink-0">
                    <button
                      onClick={() => copyRef(key)}
                      className="rounded-md p-1.5 hover:bg-accent transition-colors"
                      title="Copy reference URI"
                    >
                      {copiedRef === key ? (
                        <Check className="h-4 w-4 text-green-600" />
                      ) : (
                        <Link2 className="h-4 w-4 text-muted-foreground" />
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
        onConfirm={() =>
          deleteItem.mutate(item, {
            onSuccess: () => {
              setShowDelete(false);
              onDeleted();
            },
          })
        }
        title="Delete Item"
        description={`This will permanently delete "${vault}/${item}" and all its fields.`}
        isPending={deleteItem.isPending}
      />
    </div>
  );
}
