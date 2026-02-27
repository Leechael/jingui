import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { AppShell } from "~/components/layout/app-shell";
import { requireSettings } from "~/components/layout/auth-guard";
import { AppForm } from "~/components/apps/app-form";
import { useCreateApp } from "~/lib/mutations";

export const Route = createFileRoute("/apps/new")({
  beforeLoad: () => requireSettings(),
  component: NewAppPage,
});

function NewAppPage() {
  const navigate = useNavigate();
  const createApp = useCreateApp();

  return (
    <AppShell>
      <div className="space-y-6">
        <div>
          <h2 className="text-2xl font-bold tracking-tight">Create App</h2>
          <p className="text-sm text-muted-foreground">
            Register a new application vault.
          </p>
        </div>
        <AppForm
          onSubmit={(data) =>
            createApp.mutate(data, {
              onSuccess: (res) => navigate({ to: "/apps/$appId", params: { appId: res.vault } }),
            })
          }
          isPending={createApp.isPending}
          error={createApp.error?.message}
        />
      </div>
    </AppShell>
  );
}
