package gitea

import (
	"net"
	"net/http"
	"time"

	metadialer "github.com/go-i2p/go-meta-dialer"
)

func Dial(network, addr string) (net.Conn, error) {
	if metadialer.ANON {
		metadialer.ANON = false
	}
	return metadialer.Dial(network, addr)
}

func init() {
	// Initialize the client with a default timeout
	http.DefaultClient = &http.Client{
		Timeout: 360 * time.Second,
		Transport: &http.Transport{
			Dial: Dial,
		},
	}
}
