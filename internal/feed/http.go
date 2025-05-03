package feed

import (
	"net"
	"net/http"
	"time"
)

var httpTransport = &http.Transport{
	Proxy: http.ProxyFromEnvironment,
	DialContext: (&net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 60 * time.Second,
	}).DialContext,
	MaxIdleConns:        100,
	MaxIdleConnsPerHost: 10,
	IdleConnTimeout:     90 * time.Second,
	TLSHandshakeTimeout: 10 * time.Second,
	DisableCompression:  false,
}

var httpClient = &http.Client{
	Transport: httpTransport,
	Timeout:   30 * time.Second,
}
