package odyssey

import (
	"net"
	"net/http"
	"strings"
	"time"
)

// TrustedProxyCIDRs is loaded from TRUSTED_PROXIES env (comma-separated CIDRs).
// Only requests arriving from these addresses may set X-Forwarded-For / X-Real-IP.
// If empty, forwarded headers are ignored and RemoteAddr is used directly.
var TrustedProxyCIDRs []*net.IPNet

// Destination represents a stop on the interstellar journey.
type Destination struct {
	Index       int
	ID          string
	Name        string
	Description string
	Topic       string // Groq prompt topic for question generation
	Background  string // CSS class name for background theme
	Distance    string // human-readable distance from Earth
}

var Journey = []Destination{
	{0, "earth", "Earth", "Your home planet — the pale blue dot.", "Earth science and life on our planet", "bg-earth", "0 km"},
	{1, "moon", "Moon", "384,400 km away. The first step beyond Earth.", "the Moon, Apollo missions, and lunar science", "bg-moon", "384,400 km"},
	{2, "mars", "Mars", "Up to 401 million km. The Red Planet awaits.", "Mars, its moons, and Mars exploration", "bg-mars", "401,000,000 km"},
	{3, "asteroid-belt", "Asteroid Belt", "Between Mars and Jupiter. A ring of ancient rock.", "asteroids, the asteroid belt, and early solar system formation", "bg-asteroid", "479,000,000 km"},
	{4, "jupiter", "Jupiter", "778 million km. Largest planet in the solar system.", "Jupiter, its Great Red Spot, and its moons like Europa and Io", "bg-jupiter", "778,000,000 km"},
	{5, "saturn", "Saturn", "1.4 billion km. Rings made of ice and rock.", "Saturn, its ring system, and moons like Titan and Enceladus", "bg-saturn", "1,400,000,000 km"},
	{6, "uranus", "Uranus", "2.9 billion km. The ice giant tilted on its side.", "Uranus, its unusual axial tilt, and ice giant planets", "bg-uranus", "2,900,000,000 km"},
	{7, "neptune", "Neptune", "4.5 billion km. Winds reaching 2,100 km/h.", "Neptune, its storms, and the moon Triton", "bg-neptune", "4,500,000,000 km"},
	{8, "pluto", "Pluto / Kuiper Belt", "5.9 billion km. The dwarf planet at the edge.", "Pluto, the Kuiper Belt, and New Horizons mission findings", "bg-pluto", "5,900,000,000 km"},
	{9, "alpha-centauri", "Alpha Centauri", "4.37 light-years. The nearest star system to the Sun.", "Alpha Centauri, Proxima b, and interstellar travel", "bg-alpha-centauri", "4.37 ly"},
	{10, "sgr-a", "Sagittarius A*", "26,000 light-years. The supermassive black hole at the center of the Milky Way.", "black holes, Sagittarius A*, and galactic centers", "bg-sgr-a", "26,000 ly"},
}

// State holds a player's current odyssey progress, stored in Redis as JSON.
type State struct {
	IP              string    `json:"ip"`
	DestinationIdx  int       `json:"destination_idx"`
	HopsUsed        int       `json:"hops_used"`
	Completed       bool      `json:"completed"`
	LastActivity    time.Time `json:"last_activity"`
}

// ClientIP extracts the real client IP. Forwarded headers are only trusted when
// the immediate peer (RemoteAddr) is in TrustedProxyCIDRs — otherwise a client
// could spoof X-Forwarded-For to bypass per-IP rate limiting.
func ClientIP(r *http.Request) string {
	peerHost, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		peerHost = r.RemoteAddr
	}

	if isTrustedProxy(peerHost) {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			// Rightmost IP set by the trusted proxy is the authoritative client IP.
			// We use rightmost (last) rather than leftmost to prevent client spoofing
			// of additional XFF entries prepended before reaching our proxy.
			parts := strings.Split(xff, ",")
			ip := strings.TrimSpace(parts[len(parts)-1])
			if net.ParseIP(ip) != nil {
				return ip
			}
		}
		if xri := r.Header.Get("X-Real-IP"); xri != "" {
			if net.ParseIP(xri) != nil {
				return xri
			}
		}
	}

	return peerHost
}

func isTrustedProxy(host string) bool {
	if len(TrustedProxyCIDRs) == 0 {
		return false
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	for _, cidr := range TrustedProxyCIDRs {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

// ParseTrustedProxies parses a comma-separated list of CIDRs into TrustedProxyCIDRs.
// Call once at startup from the TRUSTED_PROXIES environment variable.
// Example: "10.0.0.0/8,172.16.0.0/12,127.0.0.1/32"
func ParseTrustedProxies(raw string) {
	TrustedProxyCIDRs = nil
	for _, s := range strings.Split(raw, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		// Accept bare IPs as /32 or /128
		if !strings.Contains(s, "/") {
			if strings.Contains(s, ":") {
				s += "/128"
			} else {
				s += "/32"
			}
		}
		_, cidr, err := net.ParseCIDR(s)
		if err == nil {
			TrustedProxyCIDRs = append(TrustedProxyCIDRs, cidr)
		}
	}
}
