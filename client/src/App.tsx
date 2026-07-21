import { useEffect, useState } from "react";
import { PlusCircle, Globe } from "lucide-react";
import { TooltipProvider } from "@/components/ui/tooltip";
import { Toaster } from "@/components/ui/sonner";
import { AppShell } from "@/components/layout/AppShell";
import { CallsPage } from "@/pages/CallsPage";
import { ContactsPage } from "@/pages/ContactsPage";
import { SchedulePage } from "@/pages/SchedulePage";
import { NotesPage } from "@/pages/NotesPage";
import { SessionPairing } from "@/components/domain/session/SessionPairing";
import { SessionHeader } from "@/components/domain/session/SessionHeader";
import { IncomingCallModal } from "@/components/domain/call/IncomingCallModal";
import { EmptyState } from "@/components/shared/EmptyState";
import { type PageId } from "@/components/layout/Sidebar";
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
  const [page, setPage] = useState<PageId>("calls");

  useEffect(() => {
    ensureSessionsWired();
    ensureCallsWired();
  }, []);

  const active = sessions.find((s) => s.id === activeId) ?? null;

  return (
    <TooltipProvider delayDuration={200}>
      <AppShell page={page} onSetPage={setPage}>
        {sessions.length === 0 ? (
          <EmptyState
            icon={<PlusCircle className="h-6 w-6" />}
            title={t("no_accounts")}
            description={t("no_accounts_desc")}
          />
        ) : (
          <div className="space-y-6">
            {active && (
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
            )}
            {page === "calls" && (
              active?.paired ? <CallsPage sid={active.id} /> : active ? <SessionPairing session={active} /> : null
            )}
            {page === "contacts" && <ContactsPage />}
            {page === "schedule" && <SchedulePage />}
            {page === "notes" && <NotesPage />}
          </div>
        )}
      </AppShell>
      <IncomingCallModal />
      <Toaster theme={theme} position="top-right" richColors closeButton />
    </TooltipProvider>
  );
};
