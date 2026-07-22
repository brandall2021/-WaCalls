import { useMutation } from "@tanstack/react-query";
import { toast } from "sonner";
import { openCall } from "@/lib/webrtc";
import { sounds } from "@/lib/sounds";
import { startCall } from "@/services/calls";
import { registerOwnConnection } from "@/stores/calls";

export const useStartCall = (sid: string, micId: string | null) =>
  useMutation({
    mutationFn: async (vars: { phone: string; record: boolean }) => {
      console.log(`[USE_START_CALL] starting call to ${vars.phone} sid=${sid}`);
      sounds.ring();
      const { call } = await startCall(sid, vars.phone, vars.record);
      console.log(`[USE_START_CALL] server returned callId=${call.callId}, opening WebRTC...`);
      const conn = await openCall(sid, call.callId, micId);
      console.log(`[USE_START_CALL] WebRTC connected, registering connection`);
      registerOwnConnection(call.callId, conn);
      sounds.stop();
      return call.callId;
    },
    onError: (e: Error) => {
      console.error(`[USE_START_CALL] error:`, e);
      sounds.busy();
      const m = e.message;
      if (m.includes("429")) toast.error("Limit reached: max concurrent calls.");
      else if (m.includes("503")) toast.error("WhatsApp not paired.");
      else toast.error(m);
    },
  });
