import { createFileRoute, redirect } from "@tanstack/react-router";
import { getSettings } from "~/lib/settings";

export const Route = createFileRoute("/")({
  beforeLoad: () => {
    const settings = getSettings();
    if (!settings) {
      throw redirect({ to: "/settings" });
    }
    throw redirect({ to: "/apps" });
  },
});
