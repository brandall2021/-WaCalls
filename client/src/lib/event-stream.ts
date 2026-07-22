import type { CallStatus } from "@/types/call";
import type { SessionInfo, SessionState } from "@/types/session";
import { getAuthToken } from "@/stores/auth";

type CallListRow = {
  sessionId: string;
  callId: string;
  owner: string | null;
  direction: "outbound" | "inbound";
  peer: string;
  startedAt: number;
  status: CallStatus;
  endedAt?: number;
  endReason?: string;
};

export type BrokerEvent =
  | { type: "session-list"; sessions: SessionInfo[] }
  | { type: "session-qr"; sessionId: string; qr: string }
  | { type: "auth-state"; sessionId: string; paired: boolean; state: SessionState; qr?: string }
  | { type: "call-list"; calls: CallListRow[] }
  | { type: "call-status"; sessionId: string; id: string; owner: string | null; status: CallStatus; peer: string; startedAt: number }
  | { type: "call-ended"; sessionId: string; id: string; owner: string | null; reason: string; endedAt: number }
  | { type: "incoming"; sessionId: string; id: string; peer: string; offeredAt: number }
  | { type: "incoming-claimed"; sessionId: string; id: string; owner: string };

type Listener = (ev: BrokerEvent) => void;

class EventStream {
  #es: EventSource | null = null;
  #listeners = new Set<Listener>();
  #clientId: string = "";
  #reconnectTimer: ReturnType<typeof setTimeout> | null = null;

  connect(clientId: string): void {
    if (this.#es) return;
    this.#clientId = clientId;
    const token = getAuthToken();
    const params = new URLSearchParams({ clientId });
    if (token) params.set("token", token);
    console.log(`[SSE] connecting with clientId=${clientId} token=${token ? token.substring(0, 20) + '...' : 'NONE'}`);
    this.#es = new EventSource(`/api/events?${params.toString()}`);
    this.#es.onopen = () => console.log(`[SSE] connected`);
    this.#es.onmessage = (ev) => {
      try {
        const parsed: BrokerEvent = JSON.parse(ev.data);
        console.log(`[SSE] event: ${parsed.type}`, parsed);
        for (const l of this.#listeners) l(parsed);
      } catch {}
    };
    this.#es.onerror = (e) => {
      console.warn(`[SSE] error, readyState=${this.#es?.readyState}`, e);
      this.#es?.close();
      this.#es = null;
      if (this.#reconnectTimer) clearTimeout(this.#reconnectTimer);
      this.#reconnectTimer = setTimeout(() => {
        this.#reconnectTimer = null;
        if (!this.#es && this.#clientId) {
          console.log(`[SSE] reconnecting...`);
          this.connect(this.#clientId);
        }
      }, 2000);
    };
  }

  on(l: Listener): () => void {
    this.#listeners.add(l);
    return () => this.#listeners.delete(l);
  }

  close(): void {
    if (this.#reconnectTimer) {
      clearTimeout(this.#reconnectTimer);
      this.#reconnectTimer = null;
    }
    this.#es?.close();
    this.#es = null;
    this.#clientId = "";
  }
}

export const eventStream = new EventStream();
