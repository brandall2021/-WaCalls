package call

import (
	"wacalls/internal/voip/core"
	"wacalls/internal/voip/transport"
)

type RelayTransport interface {
	SetSsrc(ssrc uint32)
	SetSubscriptionSsrc(ssrc uint32)
	SetParticipantPIDs(selfPid, peerPid *int)
	SetOnConnected(fn func(ip string, port int))
	SetOnReceive(fn func(data []byte))
	ResendSubscriptions()
	ConfigureRelays(relays []transport.RelayConfig)
	Broadcast(data []byte)
	HasConnection() bool
	ConnectedCount() int
	Cleanup()
}

var _ RelayTransport = (*transport.SctpRelayManager)(nil)

func (m *CallManager) onRelayConnected() {
	m.mu.Lock()
	call := m.currentCall
	if call == nil {
		m.mu.Unlock()
		return
	}
	m.log.Info("onRelayConnected fired", "call_id", call.CallID, "state", string(call.StateData.State),
		"relay_connected", m.relay.HasConnection(), "connected_count", m.relay.ConnectedCount())

	switch call.StateData.State {
	case core.CallStateConnecting, core.CallStateInitiating:
		if call.StateData.State == core.CallStateConnecting {
			// Only transition to MediaConnected if we were in Connecting state
			if err := call.ApplyTransition(Transition{Type: TransitionMediaConnected}); err != nil {
				m.log.Warn("relay connected but transition failed", "call_id", call.CallID, "err", err)
				m.mu.Unlock()
				return
			}
			m.emitState()
		}
		// Send an initial audio frame immediately to signal readiness to WhatsApp's relay.
		// The relay may need to see outgoing audio before it starts forwarding incoming audio.
		m.sendInitialAudioFrameLocked()
		m.startSilenceKeepaliveLocked()
		m.log.Info("relay connected → ready", "call_id", call.CallID, "state", string(call.StateData.State))
	case core.CallStateRinging, core.CallStateIncomingRinging:
		m.log.Info("onRelayConnected: relay ready during ringing, waiting for accept", "call_id", call.CallID)
	default:
		m.log.Warn("onRelayConnected: unexpected state", "call_id", call.CallID, "state", string(call.StateData.State))
	}
	m.mu.Unlock()
}

func buildRelayConfigs(endpoints []core.RelayEndpoint) []transport.RelayConfig {
	seen := map[string]bool{}
	var relays []transport.RelayConfig
	for _, ep := range endpoints {
		if ep.Protocol != 0 {
			continue
		}
		if ep.Key == "" || ep.RawToken == nil {
			continue
		}
		key := ep.IP
		if seen[key] {
			continue
		}
		seen[key] = true
		name := ep.RelayName
		if name == "" {
			name = ep.IP
		}
		relays = append(relays, transport.RelayConfig{
			IP: ep.IP, Port: 3478, Token: ep.Token, AuthToken: ep.AuthToken,
			RawAuthToken: ep.RawAuthToken, RawToken: ep.RawToken, Key: ep.Key,
			RelayID: ep.RelayID, Name: name, AuthTokenID: ep.AuthTokenID,
		})
	}
	return relays
}

func (m *CallManager) connectRelays(endpoints []core.RelayEndpoint) {
	relays := buildRelayConfigs(endpoints)
	if len(relays) == 0 {
		m.log.Error("no usable relay configs", "endpoints", len(endpoints))
		return
	}
	m.mu.Lock()
	m.relay.SetSsrc(m.selfSsrc)
	// Always use the computed peer SSRCs for the subscription. HandleCallAck
	// and HandleCallAccept compute SSRCs from actual participant JIDs, so they
	// are accurate. Setting subscriptionSsrc=0 caused the relay allocation to
	// omit the peer SSRC, preventing the relay from forwarding incoming audio.
	if len(m.peerSsrcs) > 0 && m.peerSsrcs[0] != 0 {
		m.relay.SetSubscriptionSsrc(firstSsrc(m.peerSsrcs))
	} else {
		m.relay.SetSubscriptionSsrc(0)
	}
	m.mu.Unlock()
	m.relay.ConfigureRelays(relays)
	m.log.Info("relay configured", "configs", len(relays), "connected", m.relay.ConnectedCount(),
		"actual_peer_set", m.actualPeerSet, "peer_ssrcs", m.peerSsrcs)
}

func (m *CallManager) cleanupMedia() {
	m.mu.Lock()
	codec := m.codec
	m.codec = nil
	if m.keepaliveStop != nil {
		close(m.keepaliveStop)
		m.keepaliveStop = nil
	}
	m.rtpSession = nil
	m.srtpSession = nil
	m.firstPacketSent = false
	m.initialTransportSent = false
	m.outgoingPreacceptSent = false
	m.actualPeerSet = false
	m.encodeBuf = nil
	m.encodeBufPos = 0
	m.pendingPCM = nil
	m.log.Info("media cleaned up", "pcm_recv", m.totalPCMRecv, "frames_sent", m.totalFramesSent, "relay_recv", m.totalRelayRecv)
	m.totalPCMRecv = 0
	m.totalFramesSent = 0
	m.totalRelayRecv = 0
	m.mu.Unlock()

	go m.relay.Cleanup()
	if codec != nil {
		codec.Close()
	}
}
