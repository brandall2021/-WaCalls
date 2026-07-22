package main

import (
	"log/slog"
	"sync/atomic"
	"time"

	"wacalls/internal/voip/media"

	"github.com/pion/webrtc/v4"
)

// pcmChannelLabel is the data channel the browser opens to carry raw 16 kHz mono
// Int16 LE PCM in both directions. The browser side must create it with this label.
const pcmChannelLabel = "pcm"

// Bridge is the browser-leg adapter: it carries raw PCM between the browser and
// the CallManager over a WebRTC data channel. The call core only ever sees
// []float32 PCM, so it stays unaware of the transport (no Opus here anymore).
type Bridge struct {
	pc  *webrtc.PeerConnection
	dc  atomic.Pointer[webrtc.DataChannel]
	log *slog.Logger

	// OnBrowserPCM is invoked with decoded 16 kHz mono PCM captured from the browser mic.
	OnBrowserPCM func(pcm []float32)
	// OnTerminalICE fires when the peer connection fails or closes.
	OnTerminalICE func()
}

var browserSTUN = []webrtc.ICEServer{
	{URLs: []string{"stun:stun.l.google.com:19302"}},
	{URLs: []string{"stun:stun1.l.google.com:19302"}},
	{URLs: []string{"stun:stun2.l.google.com:19302"}},
}

func NewBridge(offerSDP string, log *slog.Logger) (*Bridge, string, error) {
	log.Info("NewBridge: creating browser PeerConnection with STUN servers")
	pc, err := webrtc.NewPeerConnection(webrtc.Configuration{
		ICEServers: browserSTUN,
	})
	if err != nil {
		log.Error("NewBridge: failed to create PeerConnection", "err", err)
		return nil, "", err
	}
	br := &Bridge{pc: pc, log: log}

	pc.OnDataChannel(func(dc *webrtc.DataChannel) {
		if dc.Label() != pcmChannelLabel {
			log.Info("NewBridge: ignoring non-PCM data channel", "label", dc.Label())
			return
		}
		log.Info("NewBridge: PCM data channel received from browser")
		br.dc.Store(dc)
		dc.OnOpen(func() {
			log.Info("NewBridge: PCM data channel OPEN")
		})
		dc.OnClose(func() {
			log.Info("NewBridge: PCM data channel CLOSED")
		})
		dc.OnMessage(func(msg webrtc.DataChannelMessage) {
			if cb := br.OnBrowserPCM; cb != nil && len(msg.Data) > 0 {
				cb(media.PCMInt16LEToFloat32(msg.Data))
			}
		})
	})

	pc.OnICEConnectionStateChange(func(s webrtc.ICEConnectionState) {
		log.Info("browser ICE state", "state", s.String())
		if s == webrtc.ICEConnectionStateFailed || s == webrtc.ICEConnectionStateClosed {
			if br.OnTerminalICE != nil {
				br.OnTerminalICE()
			}
		}
	})

	pc.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c != nil {
			log.Info("browser ICE candidate", "candidate", c.String())
		}
	})

	pc.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		log.Info("browser PeerConnection state", "state", s.String())
	})

	if err := pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: offerSDP}); err != nil {
		pc.Close()
		return nil, "", err
	}
	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		pc.Close()
		return nil, "", err
	}
	gatherComplete := webrtc.GatheringCompletePromise(pc)
	if err := pc.SetLocalDescription(answer); err != nil {
		pc.Close()
		return nil, "", err
	}
	<-gatherComplete

	return br, pc.LocalDescription().SDP, nil
}

// WritePCM sends 16 kHz mono float32 PCM to the browser as Int16 LE over the data
// channel. It is a no-op until the channel is open.
func (b *Bridge) WritePCM(pcm []float32) error {
	dc := b.dc.Load()
	if dc == nil || len(pcm) == 0 {
		if dc == nil {
			b.log.Debug("WritePCM: data channel nil, dropping audio", "samples", len(pcm))
		}
		return nil
	}
	return dc.Send(media.PCMFloat32ToInt16LE(pcm))
}

func (b *Bridge) Close() {
	if b.pc != nil {
		done := make(chan struct{})
		go func() {
			_ = b.pc.Close()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			b.log.Warn("Bridge.Close: pc.Close timed out")
		}
	}
}
