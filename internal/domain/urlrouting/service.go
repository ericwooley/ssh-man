package urlrouting

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	configdomain "ssh-man/internal/domain/config"
	preferencesdomain "ssh-man/internal/domain/preferences"
	serverdomain "ssh-man/internal/domain/server"
	sessiondomain "ssh-man/internal/domain/session"
)

const defaultProbeTimeout = 1200 * time.Millisecond

type PreferenceLoader interface {
	Load(context.Context) (preferencesdomain.UserPreference, error)
}

type ConfigurationLister interface {
	ListAll(context.Context) ([]configdomain.ConnectionConfiguration, error)
}

type ServerLister interface {
	List(context.Context) ([]serverdomain.Server, error)
}

type RuntimeLister interface {
	List() []sessiondomain.RuntimeSession
}

type BrowserOpener interface {
	OpenURL(context.Context, string, string) error
	LaunchThroughSOCKSURL(context.Context, string, string, string) error
}

type ResultKind string

const (
	ResultOpened          ResultKind = "opened"
	ResultCommandExecuted ResultKind = "command_executed"
	ResultNeedsChoice     ResultKind = "needs_choice"
)

type RouteChoice struct {
	ID              string `json:"id"`
	ServerID        string `json:"serverId"`
	ServerName      string `json:"serverName"`
	ConfigurationID string `json:"configurationId"`
	BrowserID       string `json:"browserId"`
}

type RouteRequest struct {
	ID      string        `json:"id"`
	URL     string        `json:"url"`
	Choices []RouteChoice `json:"choices"`
}

type Result struct {
	Kind    ResultKind    `json:"kind"`
	Request *RouteRequest `json:"request,omitempty"`
}

type Service struct {
	preferences    PreferenceLoader
	configurations ConfigurationLister
	servers        ServerLister
	runtimes       RuntimeLister
	browsers       BrowserOpener
	probe          func(context.Context, int, string, int) error
	runCommand     func(string, string) error

	mu        sync.Mutex
	pending   *RouteRequest
	presenter func(RouteRequest)
}

func NewService(
	preferences PreferenceLoader,
	configurations ConfigurationLister,
	servers ServerLister,
	runtimes RuntimeLister,
	browsers BrowserOpener,
) *Service {
	return &Service{
		preferences:    preferences,
		configurations: configurations,
		servers:        servers,
		runtimes:       runtimes,
		browsers:       browsers,
		probe:          probeSOCKSAddress,
		runCommand:     runCommandTemplate,
	}
}

func (s *Service) SetPresenter(presenter func(RouteRequest)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.presenter = presenter
}

func (s *Service) Handle(ctx context.Context, rawURL string) (Result, error) {
	parsed, err := parseWebURL(rawURL)
	if err != nil {
		return Result{}, err
	}
	preferences, err := s.preferences.Load(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("load URL routing preferences: %w", err)
	}

	for _, rule := range preferences.URLRules {
		matcher, compileErr := regexp.Compile(rule.Pattern)
		if compileErr != nil {
			return Result{}, fmt.Errorf("compile URL rule %q: %w", rule.ID, compileErr)
		}
		if !matcher.MatchString(rawURL) {
			continue
		}
		switch rule.Action {
		case preferencesdomain.URLRuleActionBrowser:
			if err := s.browsers.OpenURL(ctx, rule.BrowserID, rawURL); err != nil {
				return Result{}, err
			}
			return Result{Kind: ResultOpened}, nil
		case preferencesdomain.URLRuleActionCommand:
			if err := s.runCommand(rule.Command, rawURL); err != nil {
				return Result{}, err
			}
			return Result{Kind: ResultCommandExecuted}, nil
		}
	}

	if isLoopbackHost(parsed.Hostname()) {
		port, portErr := webPort(parsed)
		if portErr != nil {
			return Result{}, portErr
		}
		choices, choiceErr := s.reachableServers(ctx, port, preferences.ProxyBrowserID)
		if choiceErr != nil {
			return Result{}, choiceErr
		}
		switch len(choices) {
		case 1:
			choice := choices[0]
			if err := s.browsers.LaunchThroughSOCKSURL(ctx, choice.ConfigurationID, choice.BrowserID, rawURL); err != nil {
				return Result{}, err
			}
			return Result{Kind: ResultOpened}, nil
		default:
			if len(choices) > 1 {
				request := RouteRequest{ID: routeID(), URL: rawURL, Choices: choices}
				s.mu.Lock()
				s.pending = &request
				presenter := s.presenter
				s.mu.Unlock()
				if presenter != nil {
					presenter(request)
				}
				return Result{Kind: ResultNeedsChoice, Request: &request}, nil
			}
		}
	}

	if err := s.browsers.OpenURL(ctx, preferences.DefaultBrowserID, rawURL); err != nil {
		return Result{}, err
	}
	return Result{Kind: ResultOpened}, nil
}

func (s *Service) Pending() (RouteRequest, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pending == nil {
		return RouteRequest{}, false
	}
	return cloneRequest(*s.pending), true
}

func (s *Service) ResolveChoice(ctx context.Context, requestID, choiceID string) error {
	s.mu.Lock()
	if s.pending == nil || s.pending.ID != requestID {
		s.mu.Unlock()
		return fmt.Errorf("the URL routing request is no longer available")
	}
	request := cloneRequest(*s.pending)
	var selected *RouteChoice
	for index := range request.Choices {
		if request.Choices[index].ID == choiceID {
			selected = &request.Choices[index]
			break
		}
	}
	if selected == nil {
		s.mu.Unlock()
		return fmt.Errorf("the selected URL route is not available")
	}
	s.pending = nil
	s.mu.Unlock()

	return s.browsers.LaunchThroughSOCKSURL(
		ctx,
		selected.ConfigurationID,
		selected.BrowserID,
		request.URL,
	)
}

func (s *Service) DismissChoice(requestID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pending != nil && s.pending.ID == requestID {
		s.pending = nil
	}
}

func (s *Service) reachableServers(ctx context.Context, targetPort int, browserID string) ([]RouteChoice, error) {
	configurations, err := s.configurations.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("list browser proxies: %w", err)
	}
	servers, err := s.servers.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list servers for URL routing: %w", err)
	}
	serverNames := make(map[string]string, len(servers))
	for _, server := range servers {
		serverNames[server.ID] = server.Name
	}
	runtimeByConfiguration := make(map[string]sessiondomain.RuntimeSession)
	for _, runtimeState := range s.runtimes.List() {
		runtimeByConfiguration[runtimeState.ConfigurationID] = runtimeState
	}

	type probeCandidate struct {
		choice    RouteChoice
		socksPort int
	}
	candidates := make([]probeCandidate, 0)
	for _, configuration := range configurations {
		if !configdomain.IsManagedSOCKSConfigurationID(configuration.ID) {
			continue
		}
		runtimeState, exists := runtimeByConfiguration[configuration.ID]
		if !exists || runtimeState.Status != sessiondomain.StatusConnected || runtimeState.BoundPort < 1 {
			continue
		}
		name := serverNames[configuration.ServerID]
		if name == "" {
			name = configuration.ServerID
		}
		candidates = append(candidates, probeCandidate{
			choice: RouteChoice{
				ID:              configuration.ID,
				ServerID:        configuration.ServerID,
				ServerName:      name,
				ConfigurationID: configuration.ID,
				BrowserID:       browserID,
			},
			socksPort: runtimeState.BoundPort,
		})
	}

	var wait sync.WaitGroup
	var resultMu sync.Mutex
	reachable := make([]RouteChoice, 0, len(candidates))
	for _, candidate := range candidates {
		candidate := candidate
		wait.Add(1)
		go func() {
			defer wait.Done()
			probeCtx, cancel := context.WithTimeout(ctx, defaultProbeTimeout)
			defer cancel()
			if err := s.probe(probeCtx, candidate.socksPort, "127.0.0.1", targetPort); err != nil {
				return
			}
			resultMu.Lock()
			reachable = append(reachable, candidate.choice)
			resultMu.Unlock()
		}()
	}
	wait.Wait()
	sort.Slice(reachable, func(i, j int) bool {
		if reachable[i].ServerName == reachable[j].ServerName {
			return reachable[i].ServerID < reachable[j].ServerID
		}
		return reachable[i].ServerName < reachable[j].ServerName
	})
	return reachable, nil
}

func parseWebURL(rawURL string) (*url.URL, error) {
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("URL must use http or https")
	}
	if parsed.Hostname() == "" || parsed.User != nil {
		return nil, fmt.Errorf("URL must include a host and must not include credentials")
	}
	return parsed, nil
}

func isLoopbackHost(host string) bool {
	switch strings.ToLower(strings.TrimSuffix(host, ".")) {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}

func webPort(parsed *url.URL) (int, error) {
	if parsed.Port() == "" {
		if parsed.Scheme == "https" {
			return 443, nil
		}
		return 80, nil
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("URL port must be between 1 and 65535")
	}
	return port, nil
}

func cloneRequest(request RouteRequest) RouteRequest {
	request.Choices = append([]RouteChoice(nil), request.Choices...)
	return request
}

func routeID() string {
	buffer := make([]byte, 12)
	if _, err := rand.Read(buffer); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(buffer)
}

func probeSOCKSAddress(ctx context.Context, socksPort int, host string, port int) error {
	dialer := net.Dialer{}
	connection, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(socksPort)))
	if err != nil {
		return err
	}
	defer connection.Close()

	deadline := time.Now().Add(defaultProbeTimeout)
	if contextDeadline, ok := ctx.Deadline(); ok && contextDeadline.Before(deadline) {
		deadline = contextDeadline
	}
	if err := connection.SetDeadline(deadline); err != nil {
		return err
	}
	if _, err := connection.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		return err
	}
	greeting := make([]byte, 2)
	if _, err := io.ReadFull(connection, greeting); err != nil {
		return err
	}
	if greeting[0] != 0x05 || greeting[1] != 0x00 {
		return fmt.Errorf("SOCKS proxy rejected unauthenticated connection")
	}

	request, err := socksConnectRequest(host, port)
	if err != nil {
		return err
	}
	if _, err := connection.Write(request); err != nil {
		return err
	}
	header := make([]byte, 4)
	if _, err := io.ReadFull(connection, header); err != nil {
		return err
	}
	if header[0] != 0x05 || header[1] != 0x00 {
		return fmt.Errorf("remote port is closed")
	}
	return discardSOCKSReplyAddress(connection, header[3])
}

func socksConnectRequest(host string, port int) ([]byte, error) {
	request := []byte{0x05, 0x01, 0x00}
	ip := net.ParseIP(host)
	switch {
	case ip != nil && ip.To4() != nil:
		request = append(request, 0x01)
		request = append(request, ip.To4()...)
	case ip != nil && ip.To16() != nil:
		request = append(request, 0x04)
		request = append(request, ip.To16()...)
	default:
		if len(host) > 255 {
			return nil, fmt.Errorf("target host is too long")
		}
		request = append(request, 0x03, byte(len(host)))
		request = append(request, host...)
	}
	return append(request, byte(port>>8), byte(port)), nil
}

func discardSOCKSReplyAddress(reader io.Reader, addressType byte) error {
	length := 0
	switch addressType {
	case 0x01:
		length = 4 + 2
	case 0x04:
		length = 16 + 2
	case 0x03:
		size := []byte{0}
		if _, err := io.ReadFull(reader, size); err != nil {
			return err
		}
		length = int(size[0]) + 2
	default:
		return fmt.Errorf("unsupported SOCKS address type %d", addressType)
	}
	_, err := io.CopyN(io.Discard, reader, int64(length))
	return err
}
