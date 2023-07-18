package proxy

import (
	"net/http/httputil"
	"net/url"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type Server struct {
	Endpoint string     // the target server's endpoint
	Echo     *echo.Echo // the echo server for reverse proxy
}

func NewServer(endpoint string) *Server {
	s := &Server{
		Endpoint: endpoint,
		Echo:     echo.New(),
	}

	s.Echo.Debug = true
	target, _ := url.Parse(s.Endpoint)

	s.Echo.Use(middleware.Logger())
	s.Echo.Use(middleware.Recover())

	proxy := httputil.NewSingleHostReverseProxy(target)

	// Handler for all other cases.
	s.Echo.Any("/*", func(c echo.Context) error {
		req := c.Request()
		req.Host = target.Host
		req.URL.Host = target.Host
		req.URL.Scheme = target.Scheme
		s.Echo.Logger.Info(req)
		proxy.ServeHTTP(c.Response(), c.Request())
		return nil
	})

	s.Echo.Logger.Infof("Create the reverse proxy for %s", s.Endpoint)

	return s
}

func (s *Server) Start(address string) error {
	return s.Echo.Start(address)
}
