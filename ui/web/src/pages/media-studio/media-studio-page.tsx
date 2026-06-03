import { useMemo } from "react";
import { useAuthStore } from "@/stores/use-auth-store";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

function slugify(input: string): string {
  return input
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9_-]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 80);
}

export function MediaStudioPage() {
  const userId = useAuthStore((s) => s.userId);
  const tenantSlug = useAuthStore((s) => s.tenantSlug);
  const tenantId = useAuthStore((s) => s.tenantId);

  const tenantKey = tenantSlug || tenantId || "default";
  const userKey = userId || "anonymous";

  const workspaceKey = useMemo(() => {
    return `${slugify(tenantKey)}__${slugify(userKey)}`;
  }, [tenantKey, userKey]);

  const comfyBase = (import.meta.env.VITE_COMFYUI_BASE_URL as string | undefined)?.trim() || "http://localhost:8188";
  const comfyUrl = `${comfyBase.replace(/\/$/, "")}/?workspace=${encodeURIComponent(workspaceKey)}`;

  return (
    <div className="h-[calc(100dvh-3.5rem)] p-4 sm:p-6">
      <Card className="flex h-full flex-col">
        <CardHeader>
          <CardTitle>多媒体制作</CardTitle>
          <CardDescription>
            ComfyUI 已内嵌在当前页面。系统按租户 + 用户生成隔离 workspace。
          </CardDescription>
        </CardHeader>
        <CardContent className="flex min-h-0 flex-1 flex-col space-y-4">
          <div className="rounded-md border bg-muted/40 p-3 text-sm">
            <div><span className="font-medium">当前用户：</span>{userKey}</div>
            <div><span className="font-medium">当前租户：</span>{tenantKey}</div>
            <div><span className="font-medium">Workspace Key：</span>{workspaceKey}</div>
          </div>

          <div className="min-h-0 flex-1 overflow-hidden rounded-md border bg-white">
            <iframe
              title="ComfyUI"
              src={comfyUrl}
              className="h-full w-full border-0"
              referrerPolicy="no-referrer"
            />
          </div>

          <p className="text-xs text-muted-foreground">
            如需更强隔离（模型、队列、文件系统完全隔离），建议再配合网关反向代理将 workspace 路由到独立容器实例。
          </p>
        </CardContent>
      </Card>
    </div>
  );
}
