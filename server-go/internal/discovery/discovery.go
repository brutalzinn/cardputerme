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

func FreePort(port int) bool {
	ln, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		return false
	}
	ln.Close()
	return true
}
