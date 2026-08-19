package discovery

import (
	"net"
	"strconv"
)

func PickPort(isFree func(int) bool, start, tries int) int {
	for port := start; port < start+tries; port++ {
		if isFree(port) {
			return port
		}
	}
	return 0
}

// FreePort requires the IPv4 side specifically: "tcp" alone is satisfied by
// an IPv6-only bind when another process already holds IPv4 on that port,
// which used to leave the server reachable over ::1 but not to the
// IPv4-only Cardputer.
func FreePort(port int) bool {
	ln, err := net.Listen("tcp4", ":"+strconv.Itoa(port))
	if err != nil {
		return false
	}
	ln.Close()
	return true
}
