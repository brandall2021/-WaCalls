import { create } from "zustand";
import { eventStream, type BrokerEvent } from "@/lib/event-stream";
import { getClientId } from "@/lib/client-id";
import { queryClient, queryKeys } from "@/lib/query";
import type { OpenCall } from "@/lib/webrtc";
import type { CallSummary, IncomingPayload } from "@/types/call";

type State = {
  calls: CallSummary[];
  ownConnections: Map<string, OpenCall>;
  incoming: IncomingPayload | null;
};

export const useCalls = create<State>(() => ({
  calls: [],
  ownConnections: new Map(),
  incoming: null,
}));

let wired = false;
export const ensureCallsWired = (): void => {
  if (wired) return;
  wired = true;
  console.log(`[CALLS] wiring SSE listeners`);
  eventStream.on((ev: BrokerEvent) => {
    console.log(`[CALLS] SSE event: ${ev.type}`, ev);
    if (ev.type === "call-list") {
      console.log(`[CALLS] call-list: ${ev.calls.length} calls`, ev.calls.map(c => `${c.callId}:${c.status}`));
      useCalls.setState({ calls: ev.calls });
    } else if (ev.type === "call-status") {
      console.log(`[CALLS] call-status: ${ev.id} → ${ev.status}`);
      useCalls.setState((s) => ({
        calls: s.calls.map((c) =>
          c.callId === ev.id
            ? { ...c, sessionId: ev.sessionId, status: ev.status, peer: ev.peer, startedAt: ev.startedAt }
            : c,
        ),
      }));
    } else if (ev.type === "call-ended") {
      console.log(`[CALLS] call-ended: ${ev.id} reason=${ev.reason}`);
      useCalls.setState((s) => {
        const conn = s.ownConnections.get(ev.id);
        if (conn) {
          console.log(`[CALLS] closing WebRTC connection for ${ev.id}`);
          conn.close();
        }
        const next = new Map(s.ownConnections);
        next.delete(ev.id);
        return {
          calls: s.calls.filter((c) => c.callId !== ev.id),
          ownConnections: next,
          incoming: s.incoming?.callId === ev.id ? null : s.incoming,
        };
      });
      void queryClient.invalidateQueries({ queryKey: queryKeys.history });
    } else if (ev.type === "incoming") {
      console.log(`[CALLS] incoming call: ${ev.id} from ${ev.peer}`);
      useCalls.setState({ incoming: { sessionId: ev.sessionId, callId: ev.id, peer: ev.peer, offeredAt: ev.offeredAt } });
    } else if (ev.type === "incoming-claimed") {
      console.log(`[CALLS] incoming-claimed: ${ev.id} owner=${ev.owner}`);
      useCalls.setState((s) => (s.incoming?.callId === ev.id ? { incoming: null } : s));
    }
  });
};

export const isMine = (call: CallSummary): boolean => call.owner === getClientId();

export const registerOwnConnection = (id: string, conn: OpenCall): void => {
  useCalls.setState((s) => {
    const next = new Map(s.ownConnections);
    next.set(id, conn);
    return { ownConnections: next };
  });
};

export const clearIncoming = (): void => useCalls.setState({ incoming: null });
