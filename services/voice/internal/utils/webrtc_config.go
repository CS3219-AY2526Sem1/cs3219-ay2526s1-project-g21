package utils

import (
	"os"

	"github.com/pion/webrtc/v3"
)

// GetWebRTCConfig returns WebRTC configuration
func GetWebRTCConfig() webrtc.Configuration {
	// Get STUN servers from environment or use defaults
	stunServers := []string{
		"stun:stun.l.google.com:19302",
		"stun:stun1.l.google.com:19302",
	}

	if customSTUN := os.Getenv("STUN_SERVERS"); customSTUN != "" {
		stunServers = []string{customSTUN}
	}

	// Get TURN servers from environment
	var iceServers []webrtc.ICEServer

	// Add STUN servers
	for _, stun := range stunServers {
		iceServers = append(iceServers, webrtc.ICEServer{
			URLs: []string{stun},
		})
	}

	// Add TURN servers if configured
	if turnURL := os.Getenv("TURN_URL"); turnURL != "" {
		turnUsername := os.Getenv("TURN_USERNAME")
		turnPassword := os.Getenv("TURN_PASSWORD")

		iceServers = append(iceServers, webrtc.ICEServer{
			URLs:       []string{turnURL},
			Username:   turnUsername,
			Credential: turnPassword,
		})
	}

	return webrtc.Configuration{
		ICEServers:         iceServers,
		ICETransportPolicy: webrtc.ICETransportPolicyAll,
		BundlePolicy:       webrtc.BundlePolicyMaxBundle,
		RTCPMuxPolicy:      webrtc.RTCPMuxPolicyRequire,
	}
}
