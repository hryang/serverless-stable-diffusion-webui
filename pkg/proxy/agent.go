package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
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
	// /internal/progress requests should be handled by proxy instead of agent, so return error.
	return fmt.Errorf("internal error: agent should not receive the /internal/progress request")

	req := c.Request()
	req.Host = a.Target.Host
	req.URL.Host = a.Target.Host
	req.URL.Scheme = a.Target.Scheme

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
	state, err := a.Datastore.Get(taskId)
	if err == datastore.ErrNotFound {
		state = `{"active":false,"queued":false,"completed":false,"progress":null,"eta":null,"live_preview":null,"id_live_preview":-1,"textinfo":"Waiting..."}`
		a.Datastore.Put(taskId, state)
	} else if err != nil {
		return err
	}
	//return c.JSON(http.StatusOK, state)

	a.Proxy.ServeHTTP(c.Response(), c.Request())
	return nil
}

// updateTaskProgress get the task progress info from downstream GPUServer and update it to the DB.
func (a *Agent) updateTaskProgress(taskId string) error {
	client := &http.Client{}
	previewId := 0
	for {
		// TODO: Make the task progress url configurable.
		reqBody := fmt.Sprintf(`{"id_task":"%s","id_live_preview":%d}`, taskId, previewId)
		req, err := http.NewRequest(
			"GET", "http://sd.fc-stable-diffusion.1050834996213541.cn-hangzhou.fc.devsapp.net/internal/progress", bytes.NewBuffer([]byte(reqBody)))
		if err != nil {
			return err
		}

		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		// Update the task progress to DB.
		a.Datastore.Put(taskId, string(body))
		a.Echo.Logger.Infof("update task progress: %s", string(body))

		var m map[string]interface{}
		if err := json.Unmarshal(body, &m); err != nil {
			return err
		}

		// Get live preview id from the response as the input of the next request.
		val := m["id_live_preview"]
		if val == nil {
			return fmt.Errorf("can not find id_live_preview in /internal/progress response")
		}
		var ok bool
		previewId, ok = val.(int)
		if !ok {
			return fmt.Errorf("wrong id_live_preview: %v", previewId)
		}

		// Return if complete.
		val = m["completed"]
		if val == nil {
			return fmt.Errorf("can not find completed in /internal/progress response")
		}
		completed, ok := val.(bool)
		if !ok {
			return fmt.Errorf("wrong completed: %v", completed)
		}
		if completed {
			return nil
		}
	}
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
				// Read message.
				messageType, message, err := clientConn.ReadMessage()
				if _, ok := err.(*websocket.CloseError); ok {
					a.Echo.Logger.Infof("close the websocket connection.")
					return
				}
				if err != nil {
					a.Echo.Logger.Errorf("read from websocket client error: %v", err)
					return
				}
				a.Echo.Logger.Debugf("websocket send message: %s", string(message))

				// Parse task id from message.
				var m map[string]interface{}
				if err := json.Unmarshal(message, &m); err != nil {
					a.Echo.Logger.Errorf("unmarshal json error: %v", err)
					return
				}
				var taskId string
				if data, ok := m["data"]; ok {
					if l, ok := data.([]string); ok {
						taskId = l[0]
					} else {
						a.Echo.Logger.Errorf("invalid websocket message: %v", string(message))
					}
				}

				// Launch update-task goroutine if necessary.
				if taskId != "" {
					go func() {
						a.Echo.Logger.Infof("launch update-task goroutine for task %s", taskId)
						err := a.updateTaskProgress(taskId)
						if err != nil {
							a.Echo.Logger.Errorf("update task progress error: %v", err)
						}
					}()
				}

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
					a.Echo.Logger.Infof("close the websocket connection")
					return
				}
				if err != nil {
					a.Echo.Logger.Errorf("read from websocket server error: %v", err)
					return
				}
				a.Echo.Logger.Debugf("websocket receive response: %s", string(message))
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
