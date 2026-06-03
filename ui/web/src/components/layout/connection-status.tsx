import { useTranslation } from "react-i18next";
import { useAuthStore } from "@/stores/use-auth-store";
import { cn } from "@/lib/utils";
import { cleanVersion } from "@/lib/clean-version";

const ROLE_STYLES: Record<string, string> = {
  admin: "bg-white/20 text-white",
  owner: "bg-white/20 text-white",
  operator: "bg-white/20 text-white",
  viewer: "bg-white/20 text-white",
};

export function ConnectionStatus({ collapsed }: { collapsed?: boolean }) {
  const { t } = useTranslation("common");
  const connected = useAuthStore((s) => s.connected);
  const serverVersion = useAuthStore((s) => s.serverInfo?.version);
  const tenantName = useAuthStore((s) => s.tenantName);
  const role = useAuthStore((s) => s.role);

  return (
    <div className="space-y-1.5">
      {/* Tenant + role (expanded only) */}
      {!collapsed && tenantName && (
        <div className="flex items-center justify-between gap-1.5 text-xs overflow-hidden">
          <span className="truncate font-medium text-sidebar-foreground/90">{tenantName}</span>
          {role && (
            <span className={cn("shrink-0 rounded-full px-1.5 py-0.5 text-2xs font-medium", ROLE_STYLES[role] ?? ROLE_STYLES.viewer)}>
              {role}
            </span>
          )}
        </div>
      )}

      {/* Connection status */}
      <div className="flex items-center gap-2 text-xs text-sidebar-foreground/85 overflow-hidden">
        <span
          className={cn(
            "h-2 w-2 shrink-0 rounded-full",
            connected ? "bg-green-500" : "bg-red-500",
          )}
        />
        {!collapsed && (
          <span className="truncate">
            {connected ? t("connected") : t("disconnected")}
            {connected && serverVersion && (
              <span className="ml-1 opacity-60">· {cleanVersion(serverVersion)}</span>
            )}
          </span>
        )}
      </div>
    </div>
  );
}
