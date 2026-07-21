import { useEffect } from "react";
import { PlusCircle, Globe } from "lucide-react";
import { TooltipProvider } from "@/components/ui/tooltip";
import { Toaster } from "@/components/ui/sonner";
import { AppShell } from "@/components/layout/AppShell";
import { CallsPage } from "@/pages/CallsPage";
import { SessionPairing } from "@/components/domain/session/SessionPairing";
import { SessionHeader } from "@/components/domain/session/SessionHeader";
import { IncomingCallModal } from "@/components/domain/call/IncomingCallModal";
import { EmptyState } from "@/components/shared/EmptyState";
import { ensureSessionsWired, useSessions } from "@/stores/sessions";
import { ensureCallsWired } from "@/stores/calls";
import { useTheme } from "@/stores/theme";
import { useI18n, type Locale } from "@/lib/i18n";

const locales: Locale[] = ["en", "es", "pt"];

export const App = () => {
  const sessions = useSessions((s) => s.sessions);
  const activeId = useSessions((s) => s.activeId);
  const theme = useTheme((s) => s.theme);
  const { t, locale, setLocale } = useI18n();

  useEffect(() => {
    ensureSessionsWired();
    ensureCallsWired();
  }, []);

  const active = sessions.find((s) => s.id === activeId) ?? null;

  return (
    <TooltipProvider delayDuration={200}>
      <AppShell>
        {sessions.length === 0 ? (
          <EmptyState
            icon={<PlusCircle className="h-6 w-6" />}
            title={t("no_accounts")}
            description={t("no_accounts_desc")}
          />
        ) : active ? (
          <div className="space-y-6">
            <div className="flex items-center justify-between">
              <SessionHeader session={active} />
              <div className="flex items-center gap-2">
                <Globe className="h-4 w-4 text-muted-foreground" />
                <select
                  value={locale}
                  onChange={(e) => setLocale(e.target.value as Locale)}
                  className="h-8 rounded-md border border-input bg-transparent px-2 text-xs shadow-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
                >
                  {locales.map((l) => (
                    <option key={l} value={l}>{l.toUpperCase()}</option>
                  ))}
                </select>
              </div>
            </div>
            {active.paired ? <CallsPage sid={active.id} /> : <SessionPairing session={active} />}
          </div>
        ) : (
          <EmptyState title={t("select_account")} description={t("select_account_desc")} />
        )}
      </AppShell>
      <IncomingCallModal />
      <Toaster theme={theme} position="top-right" richColors closeButton />
    </TooltipProvider>
  );
};
