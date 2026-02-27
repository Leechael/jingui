import { redirect } from "@tanstack/react-router";
import { getSettings } from "~/lib/settings";

export function requireSettings() {
  const settings = getSettings();
  if (!settings) {
    throw redirect({ to: "/settings" });
  }
  return settings;
}
