package call

import (
	"time"
	"wacalls/internal/voip/core"
	"wacalls/internal/voip/media"
	"wacalls/internal/voip/transport"
)

func (m *CallManager) initCodec() {
	if m.codec != nil {
		return
	}
	codec, err := media.NewMLowCodec(media.DefaultCodecOptions)
	if err != nil {
		m.log.Warn("MLow codec unavailable — call will run signaling-only (no audio)", "err", err)
		return
	}
	m.codec = codec
	m.log.Info("MLow codec initialized", "frame_size", codec.FrameSize(), "sample_rate", codec.SampleRate())
}

func (m *CallManager) FeedCapturedPCM(data []float32) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.totalPCMRecv += len(data)

	if m.totalPCMRecv == len(data) || m.totalPCMRecv%16000 == 0 {
		m.log.Info("FeedCapturedPCM: received", "samples", len(data), "total", m.totalPCMRecv,
			"codec_ok", m.codec != nil, "rtp_ok", m.rtpSession != nil,
			"srtp_ok", m.srtpSession != nil, "relay_ok", m.relay.HasConnection())
	}

	if m.codec == nil {
		if m.totalPCMRecv <= 1600 {
			m.log.Debug("FeedCapturedPCM: codec nil, dropping", "samples", len(data))
		}
		return
	}
	if m.rtpSession == nil {
		if m.totalPCMRecv <= 1600 {
			m.log.Debug("FeedCapturedPCM: rtpSession nil, dropping", "samples", len(data))
		}
		return
	}
	if m.srtpSession == nil {
		if m.pendingPCM == nil {
			m.pendingPCM = make([]float32, 0, 32000)
		}
		m.pendingPCM = append(m.pendingPCM, data...)
		if len(m.pendingPCM) > 32000 {
			m.pendingPCM = m.pendingPCM[len(m.pendingPCM)-32000:]
		}
		m.log.Debug("FeedCapturedPCM: srtpSession nil, buffering", "buffered", len(m.pendingPCM))
		return
	}
	if !m.relay.HasConnection() {
		if m.pendingPCM == nil {
			m.pendingPCM = make([]float32, 0, 32000)
		}
		m.pendingPCM = append(m.pendingPCM, data...)
		if len(m.pendingPCM) > 32000 {
			m.pendingPCM = m.pendingPCM[len(m.pendingPCM)-32000:]
		}
		m.log.Debug("FeedCapturedPCM: relay not connected, buffering", "buffered", len(m.pendingPCM))
		return
	}

	// Flush buffered audio from when relay was connecting
	if len(m.pendingPCM) > 0 {
		m.log.Info("FeedCapturedPCM: flushing buffered audio", "samples", len(m.pendingPCM))
		buf := m.pendingPCM
		m.pendingPCM = nil
		m.feedPCMInternal(buf)
	}

	m.feedPCMInternal(data)
}

func (m *CallManager) feedPCMInternal(data []float32) {
	m.lastCaptureAt = time.Now()
	frameSize := m.codec.FrameSize()
	if m.encodeBuf == nil {
		m.encodeBuf = make([]float32, frameSize)
		m.encodeBufPos = 0
	}

	offset := 0
	for offset < len(data) {
		toCopy := min(len(data)-offset, frameSize-m.encodeBufPos)
		copy(m.encodeBuf[m.encodeBufPos:], data[offset:offset+toCopy])
		m.encodeBufPos += toCopy
		offset += toCopy
		if m.encodeBufPos < frameSize {
			break
		}
		frame := make([]float32, frameSize)
		copy(frame, m.encodeBuf)
		m.encodeBufPos = 0

		opus, err := m.codec.Encode(frame)
		if err != nil {
			m.log.Debug("encode error", "err", err)
			continue
		}
		m.sendOpusFrameLocked(opus)
	}
}

func (m *CallManager) sendOpusFrameLocked(opus []byte) {
	if m.rtpSession == nil || m.srtpSession == nil {
		if m.totalFramesSent == 0 {
			m.log.Debug("sendOpusFrameLocked: rtp or srtp nil", "rtp_nil", m.rtpSession == nil, "srtp_nil", m.srtpSession == nil)
		}
		return
	}
	marker := !m.firstPacketSent
	pkt := m.rtpSession.CreatePacketWithDuration(opus, m.codec.FrameSize(), marker)
	if m.debeEnabled {
		pkt.Header.Extension = true
		pkt.Header.ExtensionProfile = 0xbede
		pkt.Header.ExtensionData = nil
	}
	m.firstPacketSent = true
	m.totalFramesSent++

	if m.totalFramesSent%100 == 1 {
		m.log.Info("audio frame sent", "frames", m.totalFramesSent, "pcm_recv", m.totalPCMRecv,
			"relay_connected", m.relay.HasConnection())
	}

	srtp, err := m.srtpSession.Protect(pkt)
	if err != nil {
		m.log.Warn("srtp protect error", "err", err, "frame", m.totalFramesSent)
		return
	}
	if !m.relay.HasConnection() {
		if m.totalFramesSent <= 3 {
			m.log.Warn("sendOpusFrameLocked: relay not connected, SRTP packet dropped", "frame", m.totalFramesSent)
		}
		return
	}
	m.relay.Broadcast(srtp)
}

func (m *CallManager) startSilenceKeepaliveLocked() {
	if m.keepaliveStop != nil || m.codec == nil {
		return
	}
	stop := make(chan struct{})
	m.keepaliveStop = stop
	frameSize := m.codec.FrameSize()
	go func() {
		ticker := time.NewTicker(60 * time.Millisecond)
		defer ticker.Stop()
		silence := make([]float32, frameSize)
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				m.mu.Lock()
				ready := m.codec != nil && m.rtpSession != nil && m.srtpSession != nil && m.relay.HasConnection()
				idle := time.Since(m.lastCaptureAt) > 120*time.Millisecond
				if ready && idle {
					if opus, err := m.codec.Encode(silence); err == nil {
						m.sendOpusFrameLocked(opus)
					}
				}
				m.mu.Unlock()
			}
		}
	}()
}

// sendInitialAudioFrameLocked sends a single silence frame immediately when the
// relay connects. This signals to WhatsApp's relay that we are ready to send and
// may prompt it to start forwarding incoming audio from the peer.
func (m *CallManager) sendInitialAudioFrameLocked() {
	if m.codec == nil || m.rtpSession == nil || m.srtpSession == nil || !m.relay.HasConnection() {
		m.log.Warn("sendInitialAudioFrameLocked: skipped (not ready)",
			"codec_nil", m.codec == nil, "rtp_nil", m.rtpSession == nil,
			"srtp_nil", m.srtpSession == nil, "relay_ok", m.relay.HasConnection())
		return
	}
	silence := make([]float32, m.codec.FrameSize())
	if opus, err := m.codec.Encode(silence); err == nil {
		m.sendOpusFrameLocked(opus)
		m.log.Info("sendInitialAudioFrameLocked: sent initial silence frame", "frame", m.totalFramesSent)
	}
}

func (m *CallManager) onRelayData(data []byte) {
	if transport.IsStunPacket(data) {
		if m.totalRelayRecv == 0 && m.totalFramesSent == 0 {
			m.log.Info("onRelayData: STUN packet from relay", "len", len(data))
		}
		return
	}
	if !transport.IsRtpPacket(data) {
		if m.totalRelayRecv == 0 {
			classified := transport.ClassifyPacket(data)
			m.log.Warn("onRelayData: non-RTP packet received", "len", len(data), "first_byte", data[0], "classified", classified)
		}
		return
	}
	if len(data) < 12 {
		return
	}
	pt := data[1] & 0x7f
	if pt != core.PayloadTypeWhatsAppOpus {
		return
	}

	m.mu.Lock()
	m.totalRelayRecv++
	ssrc := uint32(data[8])<<24 | uint32(data[9])<<16 | uint32(data[10])<<8 | uint32(data[11])

	if m.totalRelayRecv <= 3 || m.totalRelayRecv%500 == 0 {
		m.log.Info("onRelayData: RTP packet from relay", "len", len(data), "pt", pt,
			"ssrc", ssrc, "seq", uint16(data[2])<<8|uint16(data[3]),
			"self_ssrc", m.selfSsrc, "peer_ssrcs", m.peerSsrcs,
			"total_recv", m.totalRelayRecv, "total_sent", m.totalFramesSent,
			"srtp_ok", m.srtpSession != nil, "codec_ok", m.codec != nil,
			"actual_peer_set", m.actualPeerSet)
	}

	if m.srtpSession == nil || m.codec == nil {
		if m.totalRelayRecv <= 3 {
			m.log.Warn("onRelayData: dropping packet, srtp or codec nil",
				"srtp_nil", m.srtpSession == nil, "codec_nil", m.codec == nil)
		}
		m.mu.Unlock()
		return
	}

	// Ignore our own audio (loopback)
	if ssrc == m.selfSsrc {
		m.mu.Unlock()
		return
	}

	// Learn the actual peer SSRC from the first received RTP packet.
	// WhatsApp uses its own SSRCs (not the speculative ones we compute from JIDs),
	// so we must update the subscription once we see the real SSRC.
	if !m.actualPeerSet {
		m.actualPeerSet = true
		if !containsSsrc(m.peerSsrcs, ssrc) {
			m.peerSsrcs = []uint32{ssrc}
			m.relay.SetSubscriptionSsrc(ssrc)
			go m.relay.ResendSubscriptions()
			m.log.Info("onRelayData: LEARNED actual peer SSRC from RTP, subscription updated",
				"new_peer_ssrc", ssrc, "self_ssrc", m.selfSsrc)
		} else {
			m.log.Info("onRelayData: first RTP SSRC matches expected peer SSRC", "ssrc", ssrc)
			// Still resend to ensure subscription is using the confirmed SSRC
			m.relay.SetSubscriptionSsrc(ssrc)
			go m.relay.ResendSubscriptions()
		}
	}
	srtp := m.srtpSession
	codec := m.codec
	m.mu.Unlock()

	pkt, err := srtp.Unprotect(data)
	if err != nil {
		if m.totalRelayRecv <= 5 {
			m.log.Warn("srtp unprotect error", "err", err, "pkt_len", len(data), "total_recv", m.totalRelayRecv)
		}
		return
	}
	if len(pkt.Payload) == 0 {
		return
	}
	pcm, err := codec.Decode(pkt.Payload)
	if err != nil {
		if m.totalRelayRecv <= 5 {
			m.log.Warn("opus decode error", "err", err)
		}
		return
	}
	pcm = media.NormalizeFrame(pcm, codec.FrameSize())
	if m.OnPeerAudio != nil {
		m.OnPeerAudio(pcm)
	} else if m.totalRelayRecv <= 3 {
		m.log.Warn("onRelayData: OnPeerAudio is nil, audio dropped")
	}
}
