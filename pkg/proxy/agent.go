package proxy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/gorilla/websocket"
	"github.com/hryang/stable-diffusion-webui-proxy/pkg/datastore"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type Agent struct {
	Target    *url.URL               // the target server address
	Echo      *echo.Echo             // the echo server for reverse proxy
	Proxy     *httputil.ReverseProxy // the underlying reverse proxy
	Datastore datastore.Datastore    // the datastore to store the task states
}

func NewAgent(endpoint string) *Agent {
	a := &Agent{
		Echo: echo.New(),
	}

	//a.Echo.Debug = true
	a.Target, _ = url.Parse(endpoint)

	a.Echo.Use(middleware.Logger())
	a.Echo.Use(middleware.Recover())

	a.Proxy = httputil.NewSingleHostReverseProxy(a.Target)

	a.Datastore = datastore.NewSQLiteDatastore(":memory:")

	a.Echo.POST("/internal/progress", a.progressHandler)
	a.Echo.GET("/queue/join", a.queueJoinHandler)

	// Handler for all other cases.
	a.Echo.Any("/*", func(c echo.Context) error {
		req := c.Request()
		req.Host = a.Target.Host
		req.URL.Host = a.Target.Host
		req.URL.Scheme = a.Target.Scheme
		a.Echo.Logger.Info(req)
		a.Proxy.ServeHTTP(c.Response(), c.Request())
		return nil
	})

	a.Echo.Logger.Infof("Create the webui agent for %s", endpoint)

	return a
}

func (a *Agent) Start(address string) error {
	return a.Echo.Start(address)
}

func (a *Agent) Close() error {
	return a.Datastore.Close()
}

func (a *Agent) progressHandler(c echo.Context) error {
	// Check whether it's a new task.

	// Submit the new task.

	// Query the existed task state.

	req := c.Request()
	req.Host = a.Target.Host
	req.URL.Host = a.Target.Host
	req.URL.Scheme = a.Target.Scheme

	// Get the response from DB.
	var body map[string]interface{}
	if err := c.Bind(&body); err != nil {
		return err
	}
	taskId := body["id_task"].(string)
	fmt.Printf("faint: %s\n", taskId)

	a.Proxy.ServeHTTP(c.Response(), c.Request())
	return nil
}

func (a *Agent) queueJoinHandler(c echo.Context) error {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	// Upgrade the client HTTP to WebSocket connection.
	clientConn, err := upgrader.Upgrade(c.Response().Writer, c.Request(), nil)
	if err != nil {
		return err
	}
	defer clientConn.Close()

	// Create the downstream server WebSocket connection.
	dialer := websocket.DefaultDialer
	downstream, _ := url.JoinPath("ws://", a.Target.Host, "queue/join")
	serverConn, _, err := dialer.Dial(downstream, nil)
	if err != nil {
		return err
	}
	defer serverConn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create goroutine to handle client-to-server request.
	go func() {
		defer cancel() // cancel context when goroutine exit
		for {
			select {
			case <-ctx.Done():
				return // when context is cancel, stop the goroutine
			default:
				messageType, message, err := clientConn.ReadMessage()
				if _, ok := err.(*websocket.CloseError); ok {
					a.Echo.Logger.Infof("Close the websocket connection.")
					return
				}
				if err != nil {
					a.Echo.Logger.Errorf("read from websocket client error: %v", err)
					return
				}
				fmt.Printf("ws-req: %s\n", string(message))
				//a.Echo.Logger.Infof("ws-req: %s", string(message))
				err = serverConn.WriteMessage(messageType, message)
				if err != nil {
					a.Echo.Logger.Errorf("write to websocket server error: %v", err)
					return
				}
			}
		}
	}()

	// Create goroutine to handle server-to-client responses.
	go func() {
		defer cancel() // cancel context when goroutine exit
		for {
			select {
			case <-ctx.Done():
				return // when context is cancel, stop the goroutine
			default:
				messageType, message, err := serverConn.ReadMessage()
				if _, ok := err.(*websocket.CloseError); ok {
					a.Echo.Logger.Infof("Close the websocket connection.")
					return
				}
				if err != nil {
					a.Echo.Logger.Errorf("read from websocket server error: %v", err)
					return
				}
				fmt.Printf("ws-resp: %s\n", string(message))
				//a.Echo.Logger.Infof("ws-resp: %s", string(message))
				err = clientConn.WriteMessage(messageType, message)
				if err != nil {
					a.Echo.Logger.Errorf("write to websocket client error: %v", err)
					return
				}
			}
		}
	}()

	// Wait for at least one goroutine finishing.
	// The echo framework handles the requests in seperated goroutines. So blocking-wait is OK.
	<-ctx.Done()

	return nil
}
