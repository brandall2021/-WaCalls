import { apiPost } from "./api";
import { float32ToInt16LE, int16LEToFloat32 } from "./pcm";
import {
  CAPTURE_PROCESSOR_NAME,
  CAPTURE_WORKLET_URL,
  PCM_CHANNEL_LABEL,
  PLAYBACK_PROCESSOR_NAME,
  PLAYBACK_WORKLET_URL,
  SAMPLE_RATE,
} from "../constants/audio";

const STUN_SERVERS: RTCIceServer[] = [
  { urls: "stun:stun.l.google.com:19302" },
  { urls: "stun:stun1.l.google.com:19302" },
  { urls: "stun:stun2.l.google.com:19302" },
];

const ICE_GATHERING_TIMEOUT_MS = 8000;

export type OpenCall = {
  pc: RTCPeerConnection;
  micStream: MediaStream;
  remoteStream: MediaStream | null;
  close: () => void;
};

function debugLog(msg: string, extra?: Record<string, unknown>) {
  const ts = new Date().toISOString();
  const line = `[webrtc ${ts}] ${msg}`;
  console.log(line, extra ?? "");
  try {
    fetch("/api/debug", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ msg, ...extra }),
      keepalive: true,
    }).catch(() => {});
  } catch {}
}

export const openCall = async (
  sid: string,
  callId: string,
  micDeviceId: string | null,
): Promise<OpenCall> => {
  debugLog("openCall start", { sid, callId, micDeviceId });

  debugLog("requesting getUserMedia");
  let micStream: MediaStream;
  try {
    micStream = await navigator.mediaDevices.getUserMedia({
      audio: micDeviceId ? { deviceId: { exact: micDeviceId } } : true,
    });
    debugLog("getUserMedia OK", {
      tracks: micStream.getTracks().length,
      settings: micStream.getAudioTracks()[0]?.getSettings(),
    });
  } catch (err) {
    debugLog("getUserMedia FAILED", { error: String(err) });
    throw err;
  }

  debugLog("creating RTCPeerConnection with STUN servers");
  const pc = new RTCPeerConnection({
    iceServers: STUN_SERVERS,
    iceCandidatePoolSize: 2,
  });

  pc.oniceconnectionstatechange = () =>
    debugLog("browser ICE state", { state: pc.iceConnectionState });
  pc.onicegatheringstatechange = () =>
    debugLog("browser ICE gathering", { state: pc.iceGatheringState });
  pc.onsignalingstatechange = () =>
    debugLog("browser signaling", { state: pc.signalingState });

  const dc = pc.createDataChannel(PCM_CHANNEL_LABEL, { ordered: true });
  dc.binaryType = "arraybuffer";
  dc.onopen = () => debugLog("DataChannel OPEN");
  dc.onclose = () => debugLog("DataChannel CLOSED");
  dc.onerror = (e) => debugLog("DataChannel ERROR", { error: String(e) });

  debugLog("creating AudioContext");
  const ctx = new AudioContext({ sampleRate: SAMPLE_RATE });
  await ctx.audioWorklet.addModule(CAPTURE_WORKLET_URL);
  await ctx.audioWorklet.addModule(PLAYBACK_WORKLET_URL);
  debugLog("AudioContext state", { state: ctx.state });
  if (ctx.state === "suspended") {
    await ctx.resume();
    debugLog("AudioContext resumed", { state: ctx.state });
  }

  let pcmSent = 0;
  const micSource = ctx.createMediaStreamSource(micStream);
  const captureNode = new AudioWorkletNode(ctx, CAPTURE_PROCESSOR_NAME);
  captureNode.port.onmessage = (e: MessageEvent<Float32Array>) => {
    if (dc.readyState === "open") {
      dc.send(float32ToInt16LE(e.data));
      pcmSent++;
      if (pcmSent % 100 === 1) {
        debugLog("PCM frames sent", { total: pcmSent });
      }
    }
  };
  micSource.connect(captureNode);
  captureNode.connect(ctx.destination);

  const playbackNode = new AudioWorkletNode(ctx, PLAYBACK_PROCESSOR_NAME);
  const streamDest = ctx.createMediaStreamDestination();
  playbackNode.connect(streamDest);
  dc.onmessage = (e: MessageEvent<ArrayBuffer>) => {
    playbackNode.port.postMessage(int16LEToFloat32(e.data));
  };

  debugLog("creating SDP offer");
  const offer = await pc.createOffer();
  await pc.setLocalDescription(offer);
  debugLog("local description set, waiting for ICE", {
    candidates: pc.localDescription?.sdp.split("\n").filter((l) => l.startsWith("a=candidate")).length ?? 0,
  });

  await new Promise<void>((resolve) => {
    if (pc.iceGatheringState === "complete") {
      debugLog("ICE gathering already complete");
      resolve();
      return;
    }
    const timer = setTimeout(() => {
      debugLog("ICE gathering TIMEOUT", {
        state: pc.iceGatheringState,
        candidates: pc.localDescription?.sdp.split("\n").filter((l) => l.startsWith("a=candidate")).length ?? 0,
      });
      resolve();
    }, ICE_GATHERING_TIMEOUT_MS);
    pc.addEventListener("icegatheringstatechange", () => {
      if (pc.iceGatheringState === "complete") {
        clearTimeout(timer);
        debugLog("ICE gathering complete");
        resolve();
      }
    });
  });

  debugLog("POSTing SDP to server");
  const { sdp_answer } = await apiPost<{ sdp_answer: string }>(
    `/api/sessions/${sid}/calls/${callId}/webrtc`,
    { sdp_offer: pc.localDescription!.sdp },
  );
  debugLog("got SDP answer from server");
  await pc.setRemoteDescription({ type: "answer", sdp: sdp_answer });
  debugLog("remote description set, WebRTC handshake complete");

  return {
    pc,
    micStream,
    remoteStream: streamDest.stream,
    close: () => {
      debugLog("openCall.close()", { pcmSent });
      try {
        micStream.getTracks().forEach((t) => t.stop());
      } catch {}
      try {
        ctx.close();
      } catch {}
      try {
        pc.close();
      } catch {}
    },
  };
};
