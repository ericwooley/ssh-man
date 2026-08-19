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

	"ssh-man/internal/domain/browsercommand"
	configdomain "ssh-man/internal/domain/config"
	preferencesdomain "ssh-man/internal/domain/preferences"
	serverdomain "ssh-man/internal/domain/server"
	sessiondomain "ssh-man/internal/domain/session"
)

const (
	defaultProbeTimeout       = 1200 * time.Millisecond
	defaultChooserTimeout     = 5 * time.Second
	defaultChooserTimeoutUnit = time.Millisecond
)

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

type BrowserDestination struct {
	ID            string
	Name          string
	SupportsProxy bool
	Command       string
	Icon          string
}

type BrowserRouter interface {
	ListDestinations(context.Context) ([]BrowserDestination, error)
	OpenURL(context.Context, string, string) error
	LaunchThroughSOCKSURL(context.Context, string, string, string) error
}

type ResultKind string

const (
	ResultNeedsChoice    ResultKind = "needs_choice"
	ResultOpenedDirectly ResultKind = "opened_directly"
)

type RouteChoiceKind string

const (
	RouteChoiceBrowser RouteChoiceKind = "browser"
	RouteChoiceProxy   RouteChoiceKind = "proxy"
	RouteChoiceCommand RouteChoiceKind = "command"
)

type RouteChoice struct {
	ID              string          `json:"id"`
	Kind            RouteChoiceKind `json:"kind"`
	Label           string          `json:"label"`
	Detail          string          `json:"detail"`
	ServerID        string          `json:"serverId,omitempty"`
	ServerName      string          `json:"serverName,omitempty"`
	ConfigurationID string          `json:"configurationId,omitempty"`
	BrowserID       string          `json:"browserId,omitempty"`
	BrowserName     string          `json:"browserName,omitempty"`
	Icon            string          `json:"icon,omitempty"`
	Command         string          `json:"-"`
}

type RouteRequest struct {
	ID                  string        `json:"id"`
	URL                 string        `json:"url"`
	DefaultChoiceID     string        `json:"defaultChoiceId"`
	TimeoutMilliseconds int           `json:"timeoutMilliseconds"`
	Choices             []RouteChoice `json:"choices"`
}

type Result struct {
	Kind    ResultKind    `json:"kind"`
	Request *RouteRequest `json:"request,omitempty"`
}

type proxyServer struct {
	ServerID        string
	ServerName      string
	ConfigurationID string
	SOCKSPort       int
	Connected       bool
}

type Service struct {
	preferences    PreferenceLoader
	configurations ConfigurationLister
	servers        ServerLister
	runtimes       RuntimeLister
	browsers       BrowserRouter
	probe          func(context.Context, int, string, int) error
	runCommand     func(string, string) error

	routeMu   sync.Mutex
	mu        sync.Mutex
	pending   *RouteRequest
	presenter func(RouteRequest)
}

func NewService(
	preferences PreferenceLoader,
	configurations ConfigurationLister,
	servers ServerLister,
	runtimes RuntimeLister,
	browsers BrowserRouter,
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
	s.routeMu.Lock()
	defer s.routeMu.Unlock()

	parsed, err := parseWebURL(rawURL)
	if err != nil {
		return Result{}, err
	}
	preferences, err := s.preferences.Load(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("load URL routing preferences: %w", err)
	}
	destinations, err := s.browsers.ListDestinations(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("list URL routing browsers: %w", err)
	}

	matchedRule, err := firstMatchingRule(preferences.URLRules, rawURL)
	if err != nil {
		return Result{}, err
	}
	if matchedRule != nil && matchedRule.OpenDirect {
		choice, available, choiceErr := routeChoiceForRule(*matchedRule, destinations)
		if choiceErr != nil {
			return Result{}, choiceErr
		}
		if available {
			if executeErr := s.executeChoice(ctx, choice, rawURL); executeErr != nil {
				return Result{}, executeErr
			}
			return Result{Kind: ResultOpenedDirectly}, nil
		}
	}

	choices := regularBrowserChoices(destinations)
	defaultChoiceID, commandChoice, err := matchingRuleDefault(matchedRule)
	if err != nil {
		return Result{}, err
	}
	if commandChoice != nil {
		choices = append([]RouteChoice{*commandChoice}, choices...)
	}

	port, hasExplicitPort, err := explicitWebPort(parsed)
	if err != nil {
		return Result{}, err
	}
	configured, err := s.configuredProxyServers(ctx)
	if err != nil {
		return Result{}, err
	}
	var reachable []proxyServer
	if hasExplicitPort {
		targetHost := parsed.Hostname()
		if isLoopbackHost(targetHost) {
			targetHost = "127.0.0.1"
		}
		reachable = s.reachableServers(ctx, configured, targetHost, port)
	}
	choices = append(choices, proxyBrowserChoices(prioritizeProxyServers(configured, reachable), destinations)...)

	if defaultChoiceID == "" && hasExplicitPort {
		defaultChoiceID = assignedPortChoice(
			preferences.URLPortAssignments,
			port,
			proxyBrowserChoices(assignmentEligibleProxyServers(configured, reachable), destinations),
		)
	}
	if defaultChoiceID == "" && len(reachable) == 1 {
		browserID := preferences.ProxyBrowserID
		if browserID == "" {
			browserID = firstProxyBrowserID(destinations)
		}
		candidate := proxyChoiceID(reachable[0].ConfigurationID, browserID)
		if choiceExists(choices, candidate) {
			defaultChoiceID = candidate
		}
	}
	if defaultChoiceID == "" {
		defaultChoiceID = regularChoiceID(preferences.DefaultBrowserID)
	}
	if !choiceExists(choices, defaultChoiceID) && len(choices) > 0 {
		defaultChoiceID = choices[0].ID
	}
	if len(choices) == 0 {
		return Result{}, fmt.Errorf("no browser is available to open the URL")
	}

	request := RouteRequest{
		ID:                  routeID(),
		URL:                 rawURL,
		DefaultChoiceID:     defaultChoiceID,
		TimeoutMilliseconds: int(defaultChooserTimeout / defaultChooserTimeoutUnit),
		Choices:             choices,
	}
	s.mu.Lock()
	s.pending = &request
	presenter := s.presenter
	s.mu.Unlock()
	if presenter != nil {
		presenter(request)
	}
	return Result{Kind: ResultNeedsChoice, Request: &request}, nil
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
	s.routeMu.Lock()
	defer s.routeMu.Unlock()

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
	s.mu.Unlock()
	if selected == nil {
		return fmt.Errorf("the selected URL route is not available")
	}

	if err := s.executeChoice(ctx, *selected, request.URL); err != nil {
		return err
	}

	s.mu.Lock()
	if s.pending != nil && s.pending.ID == requestID {
		s.pending = nil
	}
	s.mu.Unlock()
	return nil
}

func (s *Service) executeChoice(ctx context.Context, selected RouteChoice, rawURL string) error {
	switch selected.Kind {
	case RouteChoiceBrowser:
		return s.browsers.OpenURL(ctx, selected.BrowserID, rawURL)
	case RouteChoiceProxy:
		return s.browsers.LaunchThroughSOCKSURL(
			ctx,
			selected.ConfigurationID,
			selected.BrowserID,
			rawURL,
		)
	case RouteChoiceCommand:
		return s.runCommand(selected.Command, rawURL)
	default:
		return fmt.Errorf("the selected URL route is invalid")
	}
}

func (s *Service) DismissChoice(requestID string) {
	s.routeMu.Lock()
	defer s.routeMu.Unlock()

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pending != nil && s.pending.ID == requestID {
		s.pending = nil
	}
}

// WithPendingInvalidation serializes a routing-relevant preference save with
// route execution. A successful save clears the captured chooser request before
// any resolver can execute it; a failed save leaves the request available.
func (s *Service) WithPendingInvalidation(save func() error) error {
	return s.WithPreferenceUpdate(func() (bool, error) {
		return true, save()
	})
}

// WithPreferenceUpdate serializes every preference write with route creation
// and execution. The callback reports whether its successful write changed a
// routing field and therefore made the captured chooser request stale.
func (s *Service) WithPreferenceUpdate(update func() (bool, error)) error {
	s.routeMu.Lock()
	defer s.routeMu.Unlock()
	invalidatePending, err := update()
	if err != nil {
		return err
	}
	if invalidatePending {
		s.mu.Lock()
		s.pending = nil
		s.mu.Unlock()
	}
	return nil
}

func regularBrowserChoices(destinations []BrowserDestination) []RouteChoice {
	choices := make([]RouteChoice, 0, len(destinations))
	for _, destination := range destinations {
		if destination.ID == "" {
			continue
		}
		name := destination.Name
		if name == "" {
			name = destination.ID
		}
		kind := RouteChoiceBrowser
		detail := "Regular browser"
		if destination.Command != "" {
			if err := browsercommand.Validate(destination.Command); err != nil {
				continue
			}
			kind = RouteChoiceCommand
			detail = "Custom browser"
		}
		choices = append(choices, RouteChoice{
			ID:          regularChoiceID(destination.ID),
			Kind:        kind,
			Label:       name,
			Detail:      detail,
			BrowserID:   destination.ID,
			BrowserName: name,
			Icon:        destination.Icon,
			Command:     destination.Command,
		})
	}
	return choices
}

func proxyBrowserChoices(servers []proxyServer, destinations []BrowserDestination) []RouteChoice {
	choices := make([]RouteChoice, 0, len(servers)*len(destinations))
	for _, server := range servers {
		for _, destination := range destinations {
			if !destination.SupportsProxy || destination.ID == "" {
				continue
			}
			browserName := destination.Name
			if browserName == "" {
				browserName = destination.ID
			}
			choices = append(choices, RouteChoice{
				ID:              proxyChoiceID(server.ConfigurationID, destination.ID),
				Kind:            RouteChoiceProxy,
				Label:           fmt.Sprintf("%s through %s", browserName, server.ServerName),
				Detail:          "SOCKS5 proxy",
				ServerID:        server.ServerID,
				ServerName:      server.ServerName,
				ConfigurationID: server.ConfigurationID,
				BrowserID:       destination.ID,
				BrowserName:     browserName,
			})
		}
	}
	return choices
}

func firstMatchingRule(rules []preferencesdomain.URLRule, rawURL string) (*preferencesdomain.URLRule, error) {
	for index := range rules {
		matches, err := urlRuleMatches(rules[index], rawURL)
		if err != nil {
			return nil, err
		}
		if matches {
			rule := rules[index]
			return &rule, nil
		}
	}
	return nil, nil
}

func urlRuleMatches(rule preferencesdomain.URLRule, rawURL string) (bool, error) {
	switch rule.MatchMode {
	case "", preferencesdomain.URLRuleMatchRegex:
		matcher, err := regexp.Compile(rule.Pattern)
		if err != nil {
			return false, fmt.Errorf("compile URL rule %q: %w", rule.ID, err)
		}
		return matcher.MatchString(rawURL), nil
	case preferencesdomain.URLRuleMatchStartsWith:
		return strings.HasPrefix(rawURL, rule.Pattern), nil
	case preferencesdomain.URLRuleMatchEndsWith:
		return strings.HasSuffix(rawURL, rule.Pattern), nil
	case preferencesdomain.URLRuleMatchContains:
		return strings.Contains(rawURL, rule.Pattern), nil
	default:
		return false, fmt.Errorf("URL rule %q has an unsupported match mode", rule.ID)
	}
}

func matchingRuleDefault(rule *preferencesdomain.URLRule) (string, *RouteChoice, error) {
	if rule == nil {
		return "", nil, nil
	}
	switch rule.Action {
	case preferencesdomain.URLRuleActionBrowser:
		return regularChoiceID(rule.BrowserID), nil, nil
	case preferencesdomain.URLRuleActionCommand:
		if err := browsercommand.Validate(rule.Command); err != nil {
			return "", nil, nil
		}
		choice := commandChoiceForRule(*rule)
		return choice.ID, &choice, nil
	default:
		return "", nil, fmt.Errorf("URL rule %q has an unsupported action", rule.ID)
	}
}

func routeChoiceForRule(rule preferencesdomain.URLRule, destinations []BrowserDestination) (RouteChoice, bool, error) {
	switch rule.Action {
	case preferencesdomain.URLRuleActionBrowser:
		choiceID := regularChoiceID(rule.BrowserID)
		for _, choice := range regularBrowserChoices(destinations) {
			if choice.ID == choiceID {
				return choice, true, nil
			}
		}
		return RouteChoice{}, false, nil
	case preferencesdomain.URLRuleActionCommand:
		if err := browsercommand.Validate(rule.Command); err != nil {
			return RouteChoice{}, false, nil
		}
		return commandChoiceForRule(rule), true, nil
	default:
		return RouteChoice{}, false, fmt.Errorf("URL rule %q has an unsupported action", rule.ID)
	}
}

func commandChoiceForRule(rule preferencesdomain.URLRule) RouteChoice {
	return RouteChoice{
		ID:      commandChoiceID(rule.ID),
		Kind:    RouteChoiceCommand,
		Label:   "Matching URL rule",
		Detail:  fmt.Sprintf("Run rule %s", rule.ID),
		Command: rule.Command,
	}
}

func assignedPortChoice(assignments []preferencesdomain.URLPortAssignment, port int, choices []RouteChoice) string {
	for _, assignment := range assignments {
		if assignment.Port != port {
			continue
		}
		candidate := proxyChoiceID(configdomain.ManagedSOCKSConfigurationID(assignment.ServerID), assignment.BrowserID)
		if choiceExists(choices, candidate) {
			return candidate
		}
		return ""
	}
	return ""
}

func firstProxyBrowserID(destinations []BrowserDestination) string {
	for _, destination := range destinations {
		if destination.SupportsProxy {
			return destination.ID
		}
	}
	return ""
}

func choiceExists(choices []RouteChoice, id string) bool {
	if id == "" {
		return false
	}
	for _, choice := range choices {
		if choice.ID == id {
			return true
		}
	}
	return false
}

func regularChoiceID(browserID string) string {
	if browserID == "" {
		return ""
	}
	return "browser:" + browserID
}

func proxyChoiceID(configurationID, browserID string) string {
	if configurationID == "" || browserID == "" {
		return ""
	}
	return "proxy:" + configurationID + ":" + browserID
}

func commandChoiceID(ruleID string) string {
	return "command:" + ruleID
}

func (s *Service) configuredProxyServers(ctx context.Context) ([]proxyServer, error) {
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

	configured := make([]proxyServer, 0)
	for _, configuration := range configurations {
		if !configdomain.IsManagedSOCKSConfigurationID(configuration.ID) {
			continue
		}
		runtimeState, exists := runtimeByConfiguration[configuration.ID]
		connected := exists && runtimeState.Status == sessiondomain.StatusConnected && runtimeState.BoundPort > 0
		name := serverNames[configuration.ServerID]
		if name == "" {
			name = configuration.ServerID
		}
		configured = append(configured, proxyServer{
			ServerID:        configuration.ServerID,
			ServerName:      name,
			ConfigurationID: configuration.ID,
			SOCKSPort:       runtimeState.BoundPort,
			Connected:       connected,
		})
	}
	sort.Slice(configured, func(i, j int) bool {
		if configured[i].ServerName == configured[j].ServerName {
			return configured[i].ServerID < configured[j].ServerID
		}
		return configured[i].ServerName < configured[j].ServerName
	})
	return configured, nil
}

func (s *Service) reachableServers(
	ctx context.Context,
	candidates []proxyServer,
	targetHost string,
	targetPort int,
) []proxyServer {
	var wait sync.WaitGroup
	var resultMu sync.Mutex
	reachable := make([]proxyServer, 0, len(candidates))
	for _, candidate := range candidates {
		candidate := candidate
		if !candidate.Connected || candidate.SOCKSPort < 1 {
			continue
		}
		wait.Add(1)
		go func() {
			defer wait.Done()
			probeCtx, cancel := context.WithTimeout(ctx, defaultProbeTimeout)
			defer cancel()
			if err := s.probe(probeCtx, candidate.SOCKSPort, targetHost, targetPort); err != nil {
				return
			}
			resultMu.Lock()
			reachable = append(reachable, candidate)
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
	return reachable
}

func prioritizeProxyServers(configured, reachable []proxyServer) []proxyServer {
	prioritized := make([]proxyServer, 0, len(configured))
	included := make(map[string]struct{}, len(configured))
	for _, server := range reachable {
		prioritized = append(prioritized, server)
		included[server.ConfigurationID] = struct{}{}
	}
	for _, server := range configured {
		if _, exists := included[server.ConfigurationID]; exists {
			continue
		}
		prioritized = append(prioritized, server)
	}
	return prioritized
}

func assignmentEligibleProxyServers(configured, reachable []proxyServer) []proxyServer {
	eligible := append([]proxyServer(nil), reachable...)
	included := make(map[string]struct{}, len(configured))
	for _, server := range reachable {
		included[server.ConfigurationID] = struct{}{}
	}
	for _, server := range configured {
		if server.Connected {
			continue
		}
		if _, exists := included[server.ConfigurationID]; exists {
			continue
		}
		eligible = append(eligible, server)
	}
	return eligible
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

func explicitWebPort(parsed *url.URL) (int, bool, error) {
	if parsed.Port() == "" {
		return 0, false, nil
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil || port < 1 || port > 65535 {
		return 0, false, fmt.Errorf("URL port must be between 1 and 65535")
	}
	return port, true, nil
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
