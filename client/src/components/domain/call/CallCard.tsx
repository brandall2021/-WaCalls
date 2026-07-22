import { useEffect, useRef, useState } from "react";
import { PhoneOff, Signal, SignalHigh, SignalLow, SignalMedium, Mic, MicOff, StickyNote, Volume2, VolumeX } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { attachMeter } from "@/lib/audio-meter";
import { sounds } from "@/lib/sounds";
import { useCalls } from "@/stores/calls";
import { useDevices } from "@/stores/devices";
import { useNotes } from "@/stores/callNotes";
import { useEndCall } from "@/hooks/useEndCall";
import { useCallQuality } from "@/hooks/useCallQuality";
import { useRecording } from "@/hooks/useRecording";
import { useI18n } from "@/lib/i18n";
import { formatCallDuration } from "@/utils/format";
import type { CallStatus, CallSummary } from "@/types/call";

const statusVariant: Record<CallStatus, "success" | "secondary" | "muted"> = {
  connected: "success",
  ringing: "secondary",
  starting: "secondary",
  ended: "muted",
};

const Meter = ({ label, db, color }: { label: string; db: number; color?: string }) => {
  const pct = Math.max(0, Math.min(100, Math.round(((db + 60) / 60) * 100)));
  const barColor = color || (db > -50 ? "bg-green-500" : "bg-primary");
  return (
    <div className="space-y-1">
      <p className="text-xs text-muted-foreground">{label}</p>
      <div className="h-2 overflow-hidden rounded-full bg-muted">
        <div className={`h-full ${barColor} transition-all`} style={{ width: `${pct}%` }} />
      </div>
    </div>
  );
};

const QualityBadge = ({ label, value, unit }: { label: string; value: number | null; unit: string }) => (
  <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
    <span className="font-medium">{label}:</span>
    <span>{value !== null ? `${value}${unit}` : "—"}</span>
  </div>
);

const formatRecordingDuration = (s: number) => `${String(Math.floor(s / 60)).padStart(2, "0")}:${String(s % 60).padStart(2, "0")}`;

export const CallCard = ({ call }: { call: CallSummary }) => {
  const conn = useCalls((s) => s.ownConnections.get(call.callId));
  const outDeviceId = useDevices((s) => s.outId);
  const endCall = useEndCall();
  const quality = useCallQuality(conn);
  const recording = useRecording();
  const notes = useNotes((s) => s);
  const t = useI18n((s) => s.t);
  const [, force] = useState(0);
  const [micDb, setMicDb] = useState(-60);
  const [peerDb, setPeerDb] = useState(-60);
  const [showNotes, setShowNotes] = useState(false);
  const [noteText, setNoteText] = useState("");
  const audioRef = useRef<HTMLAudioElement>(null);
  const [micActive, setMicActive] = useState(false);

  useEffect(() => {
    const t = setInterval(() => force((n) => n + 1), 1000);
    return () => clearInterval(t);
  }, []);

  useEffect(() => {
    if (!conn) return;
    const offMic = attachMeter(conn.micStream, setMicDb);
    let offPeer: (() => void) | null = null;
    const wait = setInterval(() => {
      if (conn.remoteStream && audioRef.current) {
        audioRef.current.srcObject = conn.remoteStream;
        audioRef.current.play().catch(() => {});
        offPeer = attachMeter(conn.remoteStream, setPeerDb);
        clearInterval(wait);
      }
    }, 200);

    const tracks = conn.micStream.getAudioTracks();
    if (tracks.length > 0) {
      setMicActive(tracks[0].enabled && tracks[0].readyState === "live");
      tracks[0].onunmute = () => setMicActive(true);
      tracks[0].onmute = () => setMicActive(false);
      tracks[0].onended = () => setMicActive(false);
    }

    return () => {
      offMic();
      offPeer?.();
      clearInterval(wait);
    };
  }, [conn]);

  useEffect(() => {
    const el = audioRef.current as (HTMLAudioElement & { setSinkId?: (id: string) => Promise<void> }) | null;
    if (!el || !outDeviceId || typeof el.setSinkId !== "function") return;
    el.setSinkId(outDeviceId).catch(() => {});
  }, [outDeviceId, conn]);

  useEffect(() => {
    if (call.status === "ended" && recording.state === "recording") {
      recording.stop();
    }
  }, [call.status]);

  const existingNote = notes.getByCall(call.callId);

  const toggleRecording = () => {
    if (!conn) return;
    const streams: MediaStream[] = [conn.micStream];
    if (conn.remoteStream) streams.push(conn.remoteStream);
    recording.toggle(streams);
  };

  const saveNote = () => {
    if (!noteText.trim()) return;
    notes.add({
      callId: call.callId,
      contactName: call.peer,
      phone: "",
      duration: Math.floor((Date.now() - call.startedAt) / 1000),
      rating: 0,
      notes: noteText.trim(),
      tags: [],
    });
    setShowNotes(false);
    setNoteText("");
  };

  const QualityIcon = quality.rtt === null ? Signal : quality.rtt < 150 ? SignalHigh : quality.rtt < 300 ? SignalMedium : SignalLow;

  return (
    <Card>
      <CardContent className="space-y-3 p-4">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            <p className="truncate font-medium">{call.peer}</p>
            <Badge variant={statusVariant[call.status]} className="mt-1">
              {formatCallDuration(call.startedAt, call.status)}
            </Badge>
          </div>
          <div className="flex items-center gap-1">
            {conn && (
              <div className={`flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium ${
                micActive ? "bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400" : "bg-muted text-muted-foreground"
              }`}>
                <Mic className={`h-3.5 w-3.5 ${micActive ? "text-green-500" : ""}`} />
                {micActive ? "MIC ON" : "MIC OFF"}
              </div>
            )}
            {call.status === "connected" && (
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    variant={recording.state === "recording" ? "destructive" : "outline"}
                    size="icon"
                    className="h-9 w-9"
                    onClick={toggleRecording}
                    aria-label={t("record")}
                  >
                    {recording.state === "recording" ? (
                      <>
                        <MicOff className="h-4 w-4" />
                        <span className="ml-1 text-xs">{formatRecordingDuration(recording.duration)}</span>
                      </>
                    ) : (
                      <Mic className="h-4 w-4" />
                    )}
                  </Button>
                </TooltipTrigger>
                <TooltipContent>
                  {recording.state === "recording" ? t("stop_recording") : t("record")}
                </TooltipContent>
              </Tooltip>
            )}
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  variant={showNotes ? "secondary" : "outline"}
                  size="icon"
                  className="h-9 w-9"
                  onClick={() => setShowNotes(!showNotes)}
                  aria-label={t("notes")}
                >
                  <StickyNote className="h-4 w-4" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>{t("notes")}</TooltipContent>
            </Tooltip>
          </div>
        </div>

        <div className="flex items-center gap-2">
          <Button
            variant="destructive"
            size="lg"
            className="flex-1 h-11 text-base font-semibold"
            onClick={() => {
              console.log(`[CALL_CARD] END CALL clicked for ${call.callId}`);
              sounds.disconnect();
              endCall.mutate({ sid: call.sessionId, callId: call.callId });
            }}
            disabled={endCall.isPending}
          >
            <PhoneOff className="h-5 w-5 mr-2" />
            {endCall.isPending ? "Cortando..." : "CORTAR LLAMADA"}
          </Button>
        </div>

        <Meter label="Mic" db={micDb} color={micActive ? "bg-green-500" : undefined} />
        <Meter label="Peer" db={peerDb} />

        {call.status === "connected" && (
          <>
            <div className="flex items-center gap-3 rounded-lg bg-muted/50 px-3 py-2">
              <QualityIcon className="h-4 w-4 text-muted-foreground" />
              <div className="flex flex-wrap gap-x-3 gap-y-0.5">
                <QualityBadge label={t("latency")} value={quality.rtt} unit="ms" />
                <QualityBadge label={t("jitter")} value={quality.jitter} unit="ms" />
                <QualityBadge label={t("packet_loss")} value={quality.packetLoss} unit="%" />
                <QualityBadge label={t("bitrate")} value={quality.bitrate} unit="kbps" />
              </div>
            </div>
            <div className="flex items-center gap-2 text-xs">
              {micDb > -50 ? (
                <>
                  <Volume2 className="h-3.5 w-3.5 text-green-500" />
                  <span className="text-green-500 font-medium">{t("audio_flow")}</span>
                </>
              ) : (
                <>
                  <VolumeX className="h-3.5 w-3.5 text-amber-500" />
                  <span className="text-amber-500">{t("no_audio_flow")}</span>
                </>
              )}
              {peerDb > -50 ? (
                <>
                  <Volume2 className="ml-2 h-3.5 w-3.5 text-green-500" />
                  <span className="text-green-500 font-medium">{t("peer_audio_flow")}</span>
                </>
              ) : (
                <>
                  <VolumeX className="ml-2 h-3.5 w-3.5 text-amber-500" />
                  <span className="text-amber-500">{t("no_peer_audio")}</span>
                </>
              )}
            </div>
          </>
        )}

        {showNotes && (
          <div className="space-y-2 rounded-lg border bg-muted/30 p-3">
            {existingNote && (
              <p className="text-xs text-muted-foreground">{existingNote.notes}</p>
            )}
            <textarea
              value={noteText}
              onChange={(e) => setNoteText(e.target.value)}
              placeholder={t("add_note")}
              rows={2}
              className="w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm"
            />
            <Button size="sm" onClick={saveNote} disabled={!noteText.trim()}>
              {t("save")}
            </Button>
          </div>
        )}

        <audio ref={audioRef} autoPlay />
      </CardContent>
    </Card>
  );
};
