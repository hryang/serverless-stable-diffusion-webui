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
	Target                *url.URL                // the target server address
	Echo                  *echo.Echo              // the echo server for reverse proxy
	TaskProgressDatastore *datastore.TaskProgress // the datastore to store the task states
}

func NewServer(targetStr string, dbType datastore.DatastoreType, dbName string) *Server {
	s := &Server{
		Echo: echo.New(),
	}
	tpds, err := datastore.NewTaskProgress(dbType, dbName)
	if err != nil {
		panic(fmt.Errorf("create task progress datastore failed: %v", err))
	}
	s.TaskProgressDatastore = tpds

	// s.Echo.Debug = true
	s.Target, err = url.Parse(targetStr)
	if err != nil {
		panic(fmt.Errorf("parse target %s failed: %v", targetStr, err))
	}

	s.Echo.Use(middleware.Logger())
	s.Echo.Use(middleware.Recover())

	proxy := httputil.NewSingleHostReverseProxy(s.Target)

	s.Echo.POST("/internal/progress", s.progressHandler)

	// Handler for all other cases.
	s.Echo.Any("/*", func(c echo.Context) error {
		req := c.Request()
		req.Host = s.Target.Host
		req.URL.Host = s.Target.Host
		req.URL.Scheme = s.Target.Scheme
		proxy.ServeHTTP(c.Response(), c.Request())
		return nil
	})

	s.Echo.Logger.Infof("create the reverse proxy for %s", targetStr)

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
	req.Host = s.Target.Host
	req.URL.Host = s.Target.Host
	req.URL.Scheme = s.Target.Scheme

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
