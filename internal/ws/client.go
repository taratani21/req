package ws

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
)

type Message struct {
	Payload       string
	AwaitResponse bool
}

func Connect(url string, headers map[string]string, timeout time.Duration) (*websocket.Conn, *http.Response, error) {
	dialer := websocket.Dialer{
		HandshakeTimeout: timeout,
	}

	reqHeaders := http.Header{}
	for k, v := range headers {
		reqHeaders.Set(k, v)
	}

	conn, resp, err := dialer.Dial(url, reqHeaders)
	if err != nil {
		return nil, resp, fmt.Errorf("websocket connect: %w", err)
	}

	return conn, resp, nil
}

func SendMessages(conn *websocket.Conn, messages []Message, timeout time.Duration, trace io.Writer) error {
	for i, msg := range messages {
		if trace != nil {
			fmt.Fprintf(trace, "→ %s\n", msg.Payload)
		}
		if err := conn.WriteMessage(websocket.TextMessage, []byte(msg.Payload)); err != nil {
			return fmt.Errorf("sending message %d: %w", i+1, err)
		}

		if msg.AwaitResponse {
			conn.SetReadDeadline(time.Now().Add(timeout))
			_, payload, err := conn.ReadMessage()
			if err != nil {
				return fmt.Errorf("awaiting response for message %d: %w", i+1, err)
			}
			conn.SetReadDeadline(time.Time{})
			if trace != nil {
				fmt.Fprintf(trace, "← %s\n", payload)
			}
		}
	}
	return nil
}

func Interactive(conn *websocket.Conn, in io.Reader, out io.Writer, trace io.Writer) error {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	defer signal.Stop(interrupt)

	done := make(chan error, 1)

	go func() {
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				done <- err
				return
			}
			fmt.Fprintf(out, "%s\n", message)
			if trace != nil {
				fmt.Fprintf(trace, "← %s\n", message)
			}
		}
	}()

	inputCh := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(in)
		for scanner.Scan() {
			inputCh <- scanner.Text()
		}
	}()

	for {
		select {
		case line := <-inputCh:
			if trace != nil {
				fmt.Fprintf(trace, "→ %s\n", line)
			}
			if err := conn.WriteMessage(websocket.TextMessage, []byte(line)); err != nil {
				return fmt.Errorf("sending message: %w", err)
			}
		case err := <-done:
			if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				return nil
			}
			return err
		case <-interrupt:
			err := conn.WriteMessage(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
			)
			if err != nil {
				return fmt.Errorf("sending close: %w", err)
			}
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			return nil
		}
	}
}
