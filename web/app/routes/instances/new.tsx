import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { AppShell } from "~/components/layout/app-shell";
import { requireSettings } from "~/components/layout/auth-guard";
import { InstanceForm } from "~/components/instances/instance-form";
import { useRegisterInstance } from "~/lib/mutations";

export const Route = createFileRoute("/instances/new")({
  beforeLoad: () => requireSettings(),
  component: NewInstancePage,
});

function NewInstancePage() {
  const navigate = useNavigate();
  const register = useRegisterInstance();

  return (
    <AppShell>
      <div className="space-y-6">
        <div>
          <h2 className="text-2xl font-bold tracking-tight">Register Instance</h2>
          <p className="text-sm text-muted-foreground">
            Register a new TEE instance.
          </p>
        </div>
        <InstanceForm
          onSubmit={(data) =>
            register.mutate(data, {
              onSuccess: (res) =>
                navigate({ to: "/instances/$fid", params: { fid: res.fid } }),
            })
          }
          isPending={register.isPending}
          error={register.error?.message}
        />
      </div>
    </AppShell>
  );
}
