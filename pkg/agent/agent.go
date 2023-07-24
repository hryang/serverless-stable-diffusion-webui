package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hryang/stable-diffusion-webui-proxy/pkg/datastore"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type Agent struct {
	Target     *url.URL               // the target server address
	Echo       *echo.Echo             // the echo server for reverse proxy
	Proxy      *httputil.ReverseProxy // the underlying reverse proxy
	Datastore  datastore.Datastore    // the datastore to store the task states
	HttpClient *http.Client           // the http client
}

func NewAgent(targetStr string, ds datastore.Datastore) *Agent {
	a := &Agent{
		Echo:       echo.New(),
		HttpClient: &http.Client{},
		Datastore:  ds,
	}

	a.Echo.Debug = true
	a.Target, _ = url.Parse(targetStr)

	a.Echo.Use(middleware.Logger())
	a.Echo.Use(middleware.Recover())

	a.Proxy = httputil.NewSingleHostReverseProxy(a.Target)

	a.Echo.GET("/queue/join", a.queueJoinHandler)

	// Handler for all other cases.
	a.Echo.Any("/*", func(c echo.Context) error {
		req := c.Request()
		req.Host = a.Target.Host
		req.URL.Host = a.Target.Host
		req.URL.Scheme = a.Target.Scheme
		a.Proxy.ServeHTTP(c.Response(), c.Request())
		return nil
	})

	a.Echo.Logger.Infof("create the webui agent for %s", targetStr)

	return a
}

func (a *Agent) Start(address string) error {
	return a.Echo.Start(address)
}

func (a *Agent) Close() error {
	return a.Datastore.Close()
}

// updateTaskProgress get the task progress info from downstream GPUServer and update it to the DB.
func (a *Agent) updateTaskProgress(ctx context.Context, taskId string) error {
	client := a.HttpClient
	var previewId float64
	notifyDone := false
	for {
		select {
		case <-ctx.Done():
			notifyDone = true
		default:
			// Do nothing, go to the next
		}
		// TODO: Make the task progress url configurable.
		reqBody := fmt.Sprintf(`{"id_task":"%s","id_live_preview":%f}`, taskId, previewId)
		req, err := http.NewRequest(
			"POST", "http://sd.fc-stable-diffusion.1050834996213541.cn-hangzhou.fc.devsapp.net/internal/progress", bytes.NewBuffer([]byte(reqBody)))
		if err != nil {
			return err
		}

		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		// Update the task progress to DB.
		a.Datastore.Put(taskId, string(body))
		a.Echo.Logger.Debugf("update task progress: %s", string(body))

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
		if previewId, ok = val.(float64); !ok {
			return fmt.Errorf("invalid id_live_preview: %v, expect type: float64, actual type: %s", val, reflect.TypeOf(val))
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
			a.Echo.Logger.Infof("the task %s is done", taskId)
			return nil
		}
		// notifyDone means the update-progress is notified by other goroutine to finish,
		// either because the task has been aborted or succeed.
		if notifyDone {
			a.Echo.Logger.Infof("the task %s is done, either success or failed", taskId)
			return nil
		}

		time.Sleep(500 * time.Millisecond)
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
	var wg sync.WaitGroup

	// Create goroutine to handle client-to-server request.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
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
				if l, ok := data.([]interface{}); ok {
					if len(l) > 0 {
						taskId, _ = l[0].(string)
					}
				}
			}
			a.Echo.Logger.Infof("websocket message: %v", string(message))

			// Launch update-task goroutine if necessary.
			// The task launching message contains the task id, which is the first element of the "data" array.
			// For example, following two messages, the first one is the task launching message, the second one is not.
			// {"fn_index": 89, "data": ["task(yx99r25qdxzgrue)", "city, cute boy", ...], ...}
			// {"fn_index": 94, "data": ["city, cute boy, ..."], ...}
			if strings.HasPrefix(taskId, "task") {
				wg.Add(1)
				go func() {
					defer wg.Done()
					a.Echo.Logger.Infof("launch update-task goroutine for task %s", taskId)
					err := a.updateTaskProgress(ctx, taskId)
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
	}()

	// Create goroutine to handle server-to-client responses.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
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
	}()

	// Wait for all goroutines finishing.
	// The echo framework handles the requests in seperated goroutines. So blocking-wait is OK.
	wg.Wait()
	cancel() // notify the update task done

	return nil
}
