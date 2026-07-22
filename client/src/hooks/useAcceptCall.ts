import { useMutation } from "@tanstack/react-query";
import { toast } from "sonner";
import { openCall } from "@/lib/webrtc";
import { acceptCall, endCall } from "@/services/calls";
import { registerOwnConnection, clearIncoming } from "@/stores/calls";

export const useAcceptCall = (micId: string | null) =>
  useMutation({
    mutationFn: async (vars: { sid: string; callId: string }) => {
      console.log(`[USE_ACCEPT_CALL] accepting call ${vars.callId} sid=${vars.sid}`);
      const res = await acceptCall(vars.sid, vars.callId);
      console.log(`[USE_ACCEPT_CALL] server accepted, opening WebRTC...`);
      try {
        const conn = await openCall(vars.sid, res.call.callId, micId);
        console.log(`[USE_ACCEPT_CALL] WebRTC connected`);
        registerOwnConnection(res.call.callId, conn);
      } catch (wrtcErr) {
        console.error(`[USE_ACCEPT_CALL] WebRTC failed:`, wrtcErr);
        try {
          await endCall(vars.sid, res.call.callId);
        } catch {}
        throw wrtcErr;
      }
      clearIncoming();
      return res.call.callId;
    },
    onError: (e: Error) => {
      console.error(`[USE_ACCEPT_CALL] error:`, e);
      if (e.message.includes("409")) {
        clearIncoming();
        return;
      }
      toast.error(e.message);
    },
  });
