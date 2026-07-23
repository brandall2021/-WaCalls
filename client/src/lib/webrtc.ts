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

export const openCall = async (
  sid: string,
  callId: string,
  micDeviceId: string | null,
): Promise<OpenCall> => {
  console.log(`[WEBRTC] openCall sid=${sid} callId=${callId} mic=${micDeviceId}`);
  const micStream = await navigator.mediaDevices.getUserMedia({
    audio: micDeviceId ? { deviceId: { exact: micDeviceId } } : true,
  });
  console.log(`[WEBRTC] micStream tracks:`, micStream.getTracks().map(t => `${t.kind}:${t.label}`));

  const pc = new RTCPeerConnection({
    iceServers: STUN_SERVERS,
    iceCandidatePoolSize: 2,
  });
  console.log(`[WEBRTC] PeerConnection created`);

  pc.oniceconnectionstatechange = () => console.log(`[WEBRTC] ICE state: ${pc.iceConnectionState}`);
  pc.onconnectionstatechange = () => console.log(`[WEBRTC] PC state: ${pc.connectionState}`);
  pc.onicecandidate = (e) => { if (e.candidate) console.log(`[WEBRTC] ICE candidate: ${e.candidate.candidate.substring(0, 80)}...`); };

  const dc = pc.createDataChannel(PCM_CHANNEL_LABEL, { ordered: true });
  dc.bufferedAmountLowThreshold = 16 * 1024;
  dc.binaryType = "arraybuffer";
  dc.onopen = () => console.log(`[WEBRTC] DataChannel OPEN`);
  dc.onclose = () => console.log(`[WEBRTC] DataChannel CLOSED`);
  dc.onerror = (e) => console.error(`[WEBRTC] DataChannel ERROR`, e);

  const ctx = new AudioContext({ sampleRate: SAMPLE_RATE });
  await ctx.audioWorklet.addModule(CAPTURE_WORKLET_URL);
  await ctx.audioWorklet.addModule(PLAYBACK_WORKLET_URL);
  if (ctx.state === "suspended") await ctx.resume();
  console.log(`[WEBRTC] AudioContext state: ${ctx.state} sampleRate: ${ctx.sampleRate}`);

  const micSource = ctx.createMediaStreamSource(micStream);
  const captureNode = new AudioWorkletNode(ctx, CAPTURE_PROCESSOR_NAME);
  let buffer: ArrayBuffer | null = null;
  let canSend = true;
  let sentCount = 0;
  let dropCount = 0;

  const sendBuffer = () => {
    if (!buffer || dc.readyState !== "open") return;
    if (dc.bufferedAmount > 64 * 1024) {
      if (canSend) {
        canSend = false;
        console.warn(`[WEBRTC] DataChannel backpressure: bufferedAmount=${dc.bufferedAmount}, pausing send`);
      }
      return;
    }
    canSend = true;
    dc.send(buffer);
    buffer = null;
    sentCount++;
    if (sentCount % 500 === 0) console.log(`[WEBRTC] PCM sent: ${sentCount} chunks, buffered: ${dc.bufferedAmount}`);
  };

  dc.onbufferedamountlow = () => {
    canSend = true;
    if (sentCount > 0 && sentCount % 500 !== 0) {
      console.log(`[WEBRTC] DataChannel backpressure released: bufferedAmount=${dc.bufferedAmount}, sent=${sentCount}`);
    }
    sendBuffer();
  };

  // Periodic diagnostic: detect stalls
  const diagInterval = setInterval(() => {
    if (dc.readyState !== "open") {
      clearInterval(diagInterval);
      return;
    }
    console.log(`[WEBRTC] DIAG: dc.readyState=${dc.readyState} bufferedAmount=${dc.bufferedAmount} sent=${sentCount} dropped=${dropCount} canSend=${canSend}`);
  }, 5000);

  captureNode.port.onmessage = (e: MessageEvent<Float32Array>) => {
    buffer = float32ToInt16LE(e.data);
    sendBuffer();
    if (!canSend) dropCount++;
  };
  micSource.connect(captureNode);

  const playbackNode = new AudioWorkletNode(ctx, PLAYBACK_PROCESSOR_NAME);
  const streamDest = ctx.createMediaStreamDestination();
  playbackNode.connect(streamDest);
  playbackNode.connect(ctx.destination);
  let recvCount = 0;
  dc.onmessage = (e: MessageEvent<ArrayBuffer>) => {
    playbackNode.port.postMessage(int16LEToFloat32(e.data));
    recvCount++;
    if (recvCount % 500 === 0) console.log(`[WEBRTC] PCM recv: ${recvCount} chunks, size: ${e.data.byteLength}`);
  };

  const offer = await pc.createOffer();
  await pc.setLocalDescription(offer);
  console.log(`[WEBRTC] SDP offer created, ICE gathering: ${pc.iceGatheringState}`);

  await new Promise<void>((resolve) => {
    if (pc.iceGatheringState === "complete") {
      console.log(`[WEBRTC] ICE gathering already complete`);
      resolve();
      return;
    }
    const timer = setTimeout(() => {
      console.log(`[WEBRTC] ICE gathering timeout (${ICE_GATHERING_TIMEOUT_MS}ms), state: ${pc.iceGatheringState}`);
      resolve();
    }, ICE_GATHERING_TIMEOUT_MS);
    pc.addEventListener("icegatheringstatechange", () => {
      if (pc.iceGatheringState === "complete") {
        clearTimeout(timer);
        console.log(`[WEBRTC] ICE gathering complete`);
        resolve();
      }
    });
  });

  console.log(`[WEBRTC] Sending SDP offer to server /api/sessions/${sid}/calls/${callId}/webrtc`);
  const { sdp_answer } = await apiPost<{ sdp_answer: string }>(
    `/api/sessions/${sid}/calls/${callId}/webrtc`,
    { sdp_offer: pc.localDescription!.sdp },
  );
  console.log(`[WEBRTC] Got SDP answer (${sdp_answer.length} bytes), setting remote description`);
  await pc.setRemoteDescription({ type: "answer", sdp: sdp_answer });
  console.log(`[WEBRTC] Remote description set, connection should be establishing`);

  return {
    pc,
    micStream,
    remoteStream: streamDest.stream,
    close: () => {
      console.log(`[WEBRTC] close() called for callId=${callId}`);
      clearInterval(diagInterval);
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
