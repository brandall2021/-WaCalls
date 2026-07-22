import { useMutation } from "@tanstack/react-query";
import { sounds } from "@/lib/sounds";
import { endCall } from "@/services/calls";

export const useEndCall = () =>
  useMutation({
    mutationFn: async (vars: { sid: string; callId: string }) => {
      console.log(`[USE_END_CALL] ending call ${vars.callId} sid=${vars.sid}`);
      sounds.disconnect();
      try {
        await endCall(vars.sid, vars.callId);
        console.log(`[USE_END_CALL] call ${vars.callId} ended successfully`);
      } catch (e) {
        console.error(`[USE_END_CALL] error ending call:`, e);
      }
    },
  });
