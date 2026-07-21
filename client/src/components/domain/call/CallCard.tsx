import { useEffect, useRef, useState } from "react";
import { PhoneOff, Signal, SignalHigh, SignalLow, SignalMedium } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { attachMeter } from "@/lib/audio-meter";
import { sounds } from "@/lib/sounds";
import { useCalls } from "@/stores/calls";
import { useDevices } from "@/stores/devices";
import { useEndCall } from "@/hooks/useEndCall";
import { useCallQuality } from "@/hooks/useCallQuality";
import { useI18n } from "@/lib/i18n";
import { formatCallDuration } from "@/utils/format";
import type { CallStatus, CallSummary } from "@/types/call";

const statusVariant: Record<CallStatus, "success" | "secondary" | "muted"> = {
  connected: "success",
  ringing: "secondary",
  starting: "secondary",
  ended: "muted",
};

const Meter = ({ label, db }: { label: string; db: number }) => {
  const pct = Math.max(0, Math.min(100, Math.round(((db + 60) / 60) * 100)));
  return (
    <div className="space-y-1">
      <p className="text-xs text-muted-foreground">{label}</p>
      <div className="h-2 overflow-hidden rounded-full bg-muted">
        <div className="h-full bg-primary transition-all" style={{ width: `${pct}%` }} />
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

export const CallCard = ({ call }: { call: CallSummary }) => {
  const conn = useCalls((s) => s.ownConnections.get(call.callId));
  const outDeviceId = useDevices((s) => s.outId);
  const endCall = useEndCall();
  const quality = useCallQuality(conn);
  const t = useI18n((s) => s.t);
  const [, force] = useState(0);
  const [micDb, setMicDb] = useState(-60);
  const [peerDb, setPeerDb] = useState(-60);
  const audioRef = useRef<HTMLAudioElement>(null);

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
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="destructive"
                size="icon"
                onClick={() => {
                  sounds.disconnect();
                  endCall.mutate({ sid: call.sessionId, callId: call.callId });
                }}
                aria-label={t("end_call")}
              >
                <PhoneOff className="h-4 w-4" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>{t("end_call")}</TooltipContent>
          </Tooltip>
        </div>
        <Meter label="Mic" db={micDb} />
        <Meter label="Peer" db={peerDb} />
        {call.status === "connected" && (
          <div className="flex items-center gap-3 rounded-lg bg-muted/50 px-3 py-2">
            <QualityIcon className="h-4 w-4 text-muted-foreground" />
            <div className="flex flex-wrap gap-x-3 gap-y-0.5">
              <QualityBadge label={t("latency")} value={quality.rtt} unit="ms" />
              <QualityBadge label={t("jitter")} value={quality.jitter} unit="ms" />
              <QualityBadge label={t("packet_loss")} value={quality.packetLoss} unit="%" />
              <QualityBadge label={t("bitrate")} value={quality.bitrate} unit="kbps" />
            </div>
          </div>
        )}
        <audio ref={audioRef} autoPlay />
      </CardContent>
    </Card>
  );
};
