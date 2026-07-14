package control

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"
)

type Client struct {
	socketPath string
	httpClient *http.Client
}

type RemoteError struct {
	Code    string
	Message string
}

func (e *RemoteError) Error() string {
	return e.Message
}

type ProtocolMismatchError struct {
	AppVersion int
	CLIVersion int
}

func (e *ProtocolMismatchError) Error() string {
	return fmt.Sprintf("SSH Man protocol mismatch: app uses %d, CLI uses %d", e.AppVersion, e.CLIVersion)
}

func NewClient(socketPath string, timeout time.Duration) *Client {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
		},
	}
	return &Client{
		socketPath: socketPath,
		httpClient: &http.Client{Transport: transport, Timeout: timeout},
	}
}

func (c *Client) Ping(ctx context.Context) error {
	return c.Call(ctx, Request{Command: "ping"}, nil)
}

func (c *Client) Wait(ctx context.Context) error {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		if err := c.Ping(ctx); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait for SSH Man control socket %q: %w", c.socketPath, ctx.Err())
		case <-ticker.C:
		}
	}
}

func (c *Client) Call(ctx context.Context, request Request, output any) error {
	request.ProtocolVersion = ProtocolVersion
	body, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("encode control request: %w", err)
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://ssh-man/v1/command", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create control request: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")

	httpResponse, err := c.httpClient.Do(httpRequest)
	if err != nil {
		return fmt.Errorf("connect to SSH Man: %w", err)
	}
	defer httpResponse.Body.Close()

	var response Response
	if err := json.NewDecoder(httpResponse.Body).Decode(&response); err != nil {
		return fmt.Errorf("decode SSH Man response: %w", err)
	}
	if response.ProtocolVersion != ProtocolVersion {
		return &ProtocolMismatchError{AppVersion: response.ProtocolVersion, CLIVersion: ProtocolVersion}
	}
	if response.Error != nil {
		return &RemoteError{Code: response.Error.Code, Message: response.Error.Message}
	}
	if output == nil || len(response.Data) == 0 {
		return nil
	}
	if err := json.Unmarshal(response.Data, output); err != nil {
		return fmt.Errorf("decode SSH Man result: %w", err)
	}
	return nil
}
