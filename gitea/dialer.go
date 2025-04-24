package gitea

import (
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-i2p/onramp"
)

var (
	Garlic, GarlicErr = onramp.NewGarlic("git-looseleaf", "127.0.0.1:7656", onramp.OPT_WIDE)
	Onion, OnionErr   = onramp.NewOnion("git-looseleaf")
)

func Dial(network, addr string) (net.Conn, error) {
	// convert the addr to a URL
	url, err := url.Parse(addr)
	if err != nil {
		return nil, err
	}
	// get the domain name
	domain := url.Hostname()
	// get the top-level domain
	fr := strings.Split(domain, ".")
	tld := fr[len(fr)-1]
	switch tld {
	case "i2p":
		if GarlicErr != nil {
			return nil, GarlicErr
		}
		// I2P is a special case, we need to use the garlic dialer
		return Garlic.Dial("i2p", addr)
	case "onion":
		if OnionErr != nil {
			return nil, OnionErr
		}
		// Onion is a special case, we need to use the onion dialer
		return Onion.Dial("onion", addr)
	default:
		// For everything else, we can use the default dialer
		return net.Dial(network, addr)
	}
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
