package sdputil

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/pion/sdp/v3"
)

// CandidateType represents the type of an ICE candidate.
type CandidateType string

const (
	CandidateTypeHost  CandidateType = "host"
	CandidateTypeSrflx CandidateType = "srflx"
	CandidateTypeRelay CandidateType = "relay"
	CandidateTypePrflx CandidateType = "prflx"
)

// ICECandidate represents a parsed ICE candidate from an SDP.
type ICECandidate struct {
	Foundation  string
	Component   int
	Protocol    string // "udp" or "tcp"
	Priority    uint32
	IP          net.IP
	Port        int
	Type        CandidateType
	RelatedIP   net.IP // for srflx: the local IP behind NAT (raddr)
	RelatedPort int    // for srflx: the local port behind NAT (rport)
}

// ParsedSDPCandidates holds ICE candidates parsed from an SDP, grouped by type.
type ParsedSDPCandidates struct {
	HostCandidates  []ICECandidate
	SrflxCandidates []ICECandidate
	RelayCandidates []ICECandidate
	AllCandidates   []ICECandidate
}

// ParseCandidatesFromSDP parses ICE candidates from an SDP string.
// It uses pion/sdp to parse the SDP, then extracts and categorizes candidate lines.
func ParseCandidatesFromSDP(sdpStr string) (*ParsedSDPCandidates, error) {
	sd := &sdp.SessionDescription{}
	if err := sd.Unmarshal([]byte(sdpStr)); err != nil {
		return nil, fmt.Errorf("failed to parse SDP: %w", err)
	}

	result := &ParsedSDPCandidates{}
	seen := make(map[string]struct{}) // deduplicate by ip:port:type

	for _, media := range sd.MediaDescriptions {
		for _, attr := range media.Attributes {
			if !attr.IsICECandidate() {
				continue
			}
			candidate, err := parseCandidateLine(attr.Value)
			if err != nil {
				continue // skip malformed candidates
			}
			// Skip IPv6 (GB28181 is IPv4-only)
			if candidate.IP.To4() == nil {
				continue
			}
			// Deduplicate
			key := fmt.Sprintf("%s:%d:%s", candidate.IP.String(), candidate.Port, candidate.Type)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}

			switch candidate.Type {
			case CandidateTypeHost:
				result.HostCandidates = append(result.HostCandidates, *candidate)
			case CandidateTypeSrflx:
				result.SrflxCandidates = append(result.SrflxCandidates, *candidate)
			case CandidateTypeRelay:
				result.RelayCandidates = append(result.RelayCandidates, *candidate)
			}
			result.AllCandidates = append(result.AllCandidates, *candidate)
		}
	}

	return result, nil
}

// parseCandidateLine parses a single ICE candidate line per RFC 8839 format:
//
//	<foundation> <component> <protocol> <priority> <ip> <port> typ <type> [raddr <rel-addr>] [rport <rel-port>]
func parseCandidateLine(line string) (*ICECandidate, error) {
	fields := strings.Fields(line)
	if len(fields) < 8 {
		return nil, fmt.Errorf("candidate line too short: %q", line)
	}

	c := &ICECandidate{
		Foundation: fields[0],
		Protocol:   fields[2],
	}

	component, err := strconv.Atoi(fields[1])
	if err != nil {
		return nil, fmt.Errorf("invalid component %q: %w", fields[1], err)
	}
	c.Component = component

	priority, err := strconv.ParseUint(fields[3], 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid priority %q: %w", fields[3], err)
	}
	c.Priority = uint32(priority)

	ip := net.ParseIP(fields[4])
	if ip == nil {
		return nil, fmt.Errorf("invalid IP %q", fields[4])
	}
	c.IP = ip

	port, err := strconv.Atoi(fields[5])
	if err != nil {
		return nil, fmt.Errorf("invalid port %q: %w", fields[5], err)
	}
	c.Port = port

	// Parse key-value pairs starting from index 6
	// Expected: "typ" <type> ["raddr" <ip>] ["rport" <port>]
	if len(fields) < 8 || fields[6] != "typ" {
		return nil, fmt.Errorf("missing 'typ' keyword in candidate line: %q", line)
	}
	c.Type = CandidateType(fields[7])

	// Parse optional raddr/rport
	for i := 8; i+1 < len(fields); i += 2 {
		switch fields[i] {
		case "raddr":
			if raddr := net.ParseIP(fields[i+1]); raddr != nil {
				c.RelatedIP = raddr
			}
		case "rport":
			if rport, err := strconv.Atoi(fields[i+1]); err == nil {
				c.RelatedPort = rport
			}
		}
	}

	return c, nil
}
