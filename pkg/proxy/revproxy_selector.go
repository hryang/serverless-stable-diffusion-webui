package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
)

type ReverseProxy struct {
	Name   string // the name of downstream stable diffusion service
	Target *url.URL
	Proxy  *httputil.ReverseProxy
}

type ReverseProxySelector interface {
	// Select select one reverse proxy from the source for the request.
	// Select can be executed by multiple goroutines concurrently.
	Select(proxies []*ReverseProxy, req *http.Request) (*ReverseProxy, error)
}

// Usually be used for testing purposes.
type RoundRobinReverseProxySelector struct {
	i     int
	mutex sync.Mutex
}

func NewRoundRobinReverseProxySelector() *RoundRobinReverseProxySelector {
	return &RoundRobinReverseProxySelector{}
}

func (rr *RoundRobinReverseProxySelector) Select(proxies []*ReverseProxy, req *http.Request) (*ReverseProxy, error) {
	length := len(proxies)
	ret := proxies[rr.i]

	rr.mutex.Lock()
	defer rr.mutex.Unlock()
	rr.i = (rr.i + 1) % length

	return ret, nil
}
