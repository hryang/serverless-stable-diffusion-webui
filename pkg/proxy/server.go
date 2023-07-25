package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/hryang/stable-diffusion-webui-proxy/pkg/datastore"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type Server struct {
	Proxies               []*ReverseProxy // the reverse proxy for each downstream sd service
	ProxySelector         ReverseProxySelector
	Echo                  *echo.Echo              // the echo server for reverse proxy
	SDServicesDatastore   *datastore.SDServices   // the datastore for the backend stable-diffusion services
	TaskProgressDatastore *datastore.TaskProgress // the datastore to store the task states
}

func NewServer(targetStr string, dbType datastore.DatastoreType, dbName string) *Server {
	s := &Server{
		Echo: echo.New(),
	}

	// TODO: Make proxy selector configurable.
	s.ProxySelector = NewRoundRobinReverseProxySelector()

	sdsd, err := datastore.NewSDServices(dbType, dbName)
	if err != nil {
		panic(fmt.Errorf("create stable-diffusion services datastore failed: %v", err))
	}
	s.SDServicesDatastore = sdsd

	tpds, err := datastore.NewTaskProgress(dbType, dbName)
	if err != nil {
		panic(fmt.Errorf("create task progress datastore failed: %v", err))
	}
	s.TaskProgressDatastore = tpds

	// s.Echo.Debug = true
	s.Echo.Use(middleware.Logger())
	s.Echo.Use(middleware.Recover())

	services, err := s.SDServicesDatastore.ListAllServiceEndpoints()
	if err != nil {
		panic(fmt.Errorf("list all service endpoints failed: %v", err))
	}
	for _, srv := range services {
		target, err := url.Parse(srv.Endpoint)
		if err != nil {
			panic(fmt.Errorf("parse target %s failed: %v", srv.Endpoint, err))
		}
		proxy := httputil.NewSingleHostReverseProxy(target)
		s.Proxies = append(s.Proxies, &ReverseProxy{
			Name:   srv.Name,
			Target: target,
			Proxy:  proxy,
		})
		s.Echo.Logger.Infof("create reverse proxy for %s: %s", srv.Name, srv.Endpoint)
	}

	s.Echo.POST("/internal/progress", s.progressHandler)

	// Handler for all other cases.
	s.Echo.Any("/*", func(c echo.Context) error {
		req := c.Request()
		p, err := s.ProxySelector.Select(s.Proxies, req)
		if err != nil {
			return err
		}
		req.Host = p.Target.Host
		req.URL.Host = p.Target.Host
		req.URL.Scheme = p.Target.Scheme
		p.Proxy.ServeHTTP(c.Response(), c.Request())
		return nil
	})

	return s
}

func (s *Server) Start(address string) error {
	return s.Echo.Start(address)
}

func (s *Server) Close() error {
	return s.TaskProgressDatastore.Close()
}

func (s *Server) progressHandler(c echo.Context) error {
	req := c.Request()
	proxy, err := s.ProxySelector.Select(s.Proxies, req)
	if err != nil {
		return err
	}
	req.Host = proxy.Target.Host
	req.URL.Host = proxy.Target.Host
	req.URL.Scheme = proxy.Target.Scheme

	// Get task id from request.
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return err
	}
	var m map[string]interface{}
	if err := json.Unmarshal(body, &m); err != nil {
		return err
	}
	taskId := m["id_task"].(string)

	// The http body is a stream and can be read once only.
	// So we have to restore the request body for serving.
	c.Request().Body = io.NopCloser(bytes.NewBuffer(body))

	// Get task state from DB.
	state, err := s.TaskProgressDatastore.GetProgress(taskId)
	if err != nil {
		return err
	}
	if state == "" {
		// When the task is not in the datastore, which means the task has not been submitted, then we return a default response to submit the task.
		state = `{"active":false,"queued":false,"completed":false,"progress":null,"eta":null,"live_preview":null,"id_live_preview":-1,"textinfo":"Waiting..."}`
	}
	return c.Blob(http.StatusOK, "application/json", []byte(state))
}
