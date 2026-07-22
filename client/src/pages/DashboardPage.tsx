import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { LayoutDashboard, Phone, Mic, Webhook, Wifi, HardDrive } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { fetchStats, type Stats } from "@/services/stats";
import { useSessions } from "@/stores/sessions";
import { useCalls } from "@/stores/calls";
import { useI18n } from "@/lib/i18n";

const formatDuration = (ms: number) => {
  const totalSecs = Math.floor(ms / 1000);
  if (totalSecs < 60) return `${totalSecs}s`;
  const hrs = Math.floor(totalSecs / 3600);
  const mins = Math.floor((totalSecs % 3600) / 60);
  if (hrs > 0) return `${hrs}h ${mins}m`;
  return `${mins}m`;
};

const formatBytes = (bytes: number) => {
  if (bytes === 0) return "0 B";
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
};

const StatCard = ({
  icon: Icon,
  label,
  value,
  sub,
  accent,
}: {
  icon: React.ElementType;
  label: string;
  value: string | number;
  sub?: string;
  accent?: boolean;
}) => (
  <Card className={accent ? "border-primary/30 bg-primary/5" : ""}>
    <CardContent className="flex items-center gap-4 p-4">
      <div className={`flex h-10 w-10 shrink-0 items-center justify-center rounded-lg ${accent ? "bg-primary/10 text-primary" : "bg-muted text-muted-foreground"}`}>
        <Icon className="h-5 w-5" />
      </div>
      <div className="min-w-0">
        <p className="text-2xl font-bold tracking-tight">{value}</p>
        <p className="text-xs text-muted-foreground">{label}</p>
        {sub && <p className="text-xs text-muted-foreground">{sub}</p>}
      </div>
    </CardContent>
  </Card>
);

export const DashboardPage = () => {
  const t = useI18n((s) => s.t);
  const sessions = useSessions((s) => s.sessions);
  const calls = useCalls((s) => s.calls);
  const [tick, setTick] = useState(0);

  const { data: stats, isLoading } = useQuery<Stats>({
    queryKey: ["stats", tick],
    queryFn: fetchStats,
    refetchInterval: 10000,
  });

  useEffect(() => {
    const id = setInterval(() => setTick((n) => n + 1), 10000);
    return () => clearInterval(id);
  }, []);

  const activeCalls = calls.filter((c) => c.status !== "ended");

  if (isLoading && !stats) {
    return (
      <div className="mx-auto max-w-5xl space-y-6">
        <div className="flex items-center gap-2">
          <Skeleton className="h-5 w-5" />
          <Skeleton className="h-6 w-32" />
        </div>
        <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
          {Array.from({ length: 4 }).map((_, i) => (
            <Skeleton key={i} className="h-[76px] rounded-lg" />
          ))}
        </div>
        <Skeleton className="h-48 rounded-lg" />
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-5xl space-y-6">
      <div className="flex items-center gap-2">
        <LayoutDashboard className="h-5 w-5 text-primary" />
        <h2 className="text-lg font-semibold">{t("dashboard")}</h2>
      </div>

      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        <StatCard
          icon={Wifi}
          label={t("sessions")}
          value={stats?.sessions.connected ?? 0}
          sub={`${stats?.sessions.total ?? 0} ${t("total")}`}
          accent={(stats?.sessions.connected ?? 0) > 0}
        />
        <StatCard
          icon={Phone}
          label={t("active_calls")}
          value={activeCalls.length}
          accent={activeCalls.length > 0}
        />
        <StatCard
          icon={Mic}
          label={t("recordings")}
          value={stats?.recordings.total ?? 0}
          sub={formatDuration(stats?.recordings.duration_ms ?? 0)}
        />
        <StatCard
          icon={HardDrive}
          label={t("storage")}
          value={formatBytes(stats?.recordings.size_bytes ?? 0)}
        />
      </div>

      <div className="grid gap-3 lg:grid-cols-2">
        <Card>
          <CardContent className="p-4">
            <h3 className="mb-3 text-sm font-medium text-muted-foreground">{t("sessions")}</h3>
            {sessions.length === 0 ? (
              <p className="text-sm text-muted-foreground">{t("no_accounts")}</p>
            ) : (
              <div className="space-y-2">
                {sessions.map((s) => (
                  <div key={s.id} className="flex items-center justify-between rounded-lg border px-3 py-2">
                    <div className="min-w-0">
                      <p className="truncate text-sm font-medium">{s.name}</p>
                      <p className="truncate text-xs text-muted-foreground">{s.jid || "—"}</p>
                    </div>
                    <Badge variant={s.state === "open" ? "success" : s.state === "qr" ? "secondary" : "muted"}>
                      {s.state === "open" ? t("connected") : s.state === "qr" ? t("scan_qr") : t("connecting")}
                    </Badge>
                  </div>
                ))}
              </div>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardContent className="p-4">
            <h3 className="mb-3 text-sm font-medium text-muted-foreground">{t("active_calls")}</h3>
            {activeCalls.length === 0 ? (
              <p className="text-sm text-muted-foreground">{t("no_active_calls_desc")}</p>
            ) : (
              <div className="space-y-2">
                {activeCalls.map((c) => (
                  <div key={c.callId} className="flex items-center justify-between rounded-lg border px-3 py-2">
                    <div className="min-w-0">
                      <p className="truncate text-sm font-medium">{c.peer}</p>
                      <p className="text-xs text-muted-foreground">{c.direction}</p>
                    </div>
                    <Badge variant={c.status === "connected" ? "success" : "secondary"}>
                      {c.status}
                    </Badge>
                  </div>
                ))}
              </div>
            )}
          </CardContent>
        </Card>
      </div>

      {(stats?.webhooks.configs ?? 0) > 0 && (
        <Card>
          <CardContent className="p-4">
            <h3 className="mb-3 text-sm font-medium text-muted-foreground">{t("webhooks")}</h3>
            <div className="flex items-center gap-6">
              <div className="flex items-center gap-2">
                <Webhook className="h-4 w-4 text-muted-foreground" />
                <span className="text-sm">{stats?.webhooks.configs ?? 0} {t("configs")}</span>
              </div>
              <div className="flex items-center gap-2 text-sm">
                <span className="text-green-600 dark:text-green-400">{stats?.webhooks.delivered ?? 0} {t("delivered")}</span>
                {(stats?.webhooks.failed ?? 0) > 0 && (
                  <span className="text-destructive">{stats?.webhooks.failed} {t("failed")}</span>
                )}
              </div>
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  );
};
