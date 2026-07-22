import { apiPost, apiDelete } from "@/lib/api";

export const startCall = (sid: string, phone: string, record: boolean) =>
  apiPost<{ call: { callId: string } }>(`/api/sessions/${sid}/calls`, {
    phone,
    duration_ms: 300_000,
    record,
  });

export const acceptCall = (sid: string, callId: string) =>
  apiPost<{ call: { callId: string } }>(`/api/sessions/${sid}/calls/${callId}/accept`, {});

export const rejectCall = (sid: string, callId: string) =>
  apiPost<{ status: string }>(`/api/sessions/${sid}/calls/${callId}/reject`, {});

export const endCall = (sid: string, callId: string) =>
  apiDelete(`/api/sessions/${sid}/calls/${callId}`);
