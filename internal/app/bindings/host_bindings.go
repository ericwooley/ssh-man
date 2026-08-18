package bindings

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"ssh-man/internal/app/bootstrap"
	appwindow "ssh-man/internal/app/window"
	"ssh-man/internal/control"
	configdomain "ssh-man/internal/domain/config"
	portlinkdomain "ssh-man/internal/domain/portlink"
	preferencesdomain "ssh-man/internal/domain/preferences"
	serverdomain "ssh-man/internal/domain/server"
	"ssh-man/internal/platform/paths"
	"ssh-man/internal/ssh/auth"
	"ssh-man/internal/ssh/remoteport"
)

const hostOpenTimeout = 15 * time.Second

type portDiscoverer interface {
	Discover(context.Context, string) (remoteport.DashboardSnapshot, error)
}

type portLinkService interface {
	ListByServer(context.Context, string) ([]portlinkdomain.Link, error)
	Save(context.Context, portlinkdomain.Link) (portlinkdomain.Link, error)
	Delete(context.Context, string) error
}

type hostPreferenceService interface {
	Load(context.Context) (preferencesdomain.UserPreference, error)
}

type portForwarder interface {
	Open(context.Context, string, int, []string) (remoteport.Forward, error)
	DialRemote(context.Context, string, int, []string) (net.Conn, error)
	Close() error
}

type faviconFinder func(context.Context, string, remoteport.DialContextFunc) (string, error)
type proxyURLLauncher func(context.Context, string, string, string) error

type HostInitialState struct {
	Server serverdomain.Server     `json:"server"`
	Links  []portlinkdomain.Link   `json:"links"`
	Theme  preferencesdomain.Theme `json:"theme"`
}

type HostPortDiscoveryResult struct {
	Metrics         remoteport.HostMetrics            `json:"metrics"`
	Ports           []remoteport.ListeningPort        `json:"ports"`
	Applications    []remoteport.ListeningApplication `json:"applications"`
	NeedsPassphrase bool                              `json:"needsPassphrase"`
}

type HostOpenPortResult struct {
	URL string `json:"url"`
}

type HostFaviconResult struct {
	FaviconDataURL string `json:"faviconDataUrl"`
}

type HostBindings struct {
	mu           sync.Mutex
	server       serverdomain.Server
	window       *appwindow.Controller
	control      hostControlClient
	links        portLinkService
	discoverer   portDiscoverer
	forwarder    portForwarder
	remoteFor    func(serverdomain.Server) (portDiscoverer, portForwarder)
	findFavicon  faviconFinder
	openProxyURL proxyURLLauncher
	preferences  hostPreferenceService
	shutdown     func(context.Context) error
	passphrase   string
	available    map[int][]string
}

func NewHostBindings(app *bootstrap.Application, server serverdomain.Server, window *appwindow.Controller) *HostBindings {
	remoteFor := func(server serverdomain.Server) (portDiscoverer, portForwarder) {
		return remoteport.NewService(server), remoteport.NewForwarder(server)
	}
	discoverer, forwarder := remoteFor(server)
	bindings := newHostBindingsWithDependencies(
		server,
		window,
		app.PortLinkService,
		discoverer,
		forwarder,
		app.Shutdown,
	)
	bindings.preferences = app.PreferencesService
	controlClient := control.NewClient(paths.ControlSocketPath(app.ConfigDir), hostOpenTimeout)
	bindings.control = controlClient
	bindings.remoteFor = remoteFor
	bindings.openProxyURL = func(ctx context.Context, configurationID, browserID, rawURL string) error {
		return controlClient.LaunchBrowserURL(ctx, configurationID, browserID, rawURL)
	}
	return bindings
}

func newHostBindingsWithDependencies(
	server serverdomain.Server,
	window *appwindow.Controller,
	links portLinkService,
	discoverer portDiscoverer,
	forwarder portForwarder,
	shutdown func(context.Context) error,
	faviconFinders ...faviconFinder,
) *HostBindings {
	if window == nil {
		window = appwindow.New()
	}
	findFavicon := remoteport.FindFaviconWithDialer
	if len(faviconFinders) > 0 && faviconFinders[0] != nil {
		findFavicon = faviconFinders[0]
	}
	return &HostBindings{
		server:      server,
		window:      window,
		links:       links,
		discoverer:  discoverer,
		forwarder:   forwarder,
		findFavicon: findFavicon,
		shutdown:    shutdown,
		available:   map[int][]string{},
	}
}

func (bindings *HostBindings) SetContext(ctx context.Context) {
	bindings.window.SetContext(ctx)
}

func (bindings *HostBindings) InitialState() (HostInitialState, error) {
	bindings.mu.Lock()
	server := bindings.server
	bindings.mu.Unlock()
	items, err := bindings.links.ListByServer(context.Background(), server.ID)
	if err != nil {
		return HostInitialState{}, fmt.Errorf("load saved port links: %w", err)
	}
	if items == nil {
		items = []portlinkdomain.Link{}
	}
	theme := preferencesdomain.ThemeDark
	if bindings.preferences != nil {
		preference, preferenceErr := bindings.preferences.Load(context.Background())
		if preferenceErr == nil {
			theme = preference.Theme
		}
	}
	return HostInitialState{Server: server, Links: items, Theme: theme}, nil
}

func (bindings *HostBindings) DiscoverPorts(passphrase string) (HostPortDiscoveryResult, error) {
	bindings.mu.Lock()
	if passphrase == "" {
		passphrase = bindings.passphrase
	}
	bindings.mu.Unlock()

	snapshot, err := bindings.discoverer.Discover(context.Background(), passphrase)
	if errors.Is(err, auth.ErrPassphraseRequired) {
		return HostPortDiscoveryResult{
			Ports:           []remoteport.ListeningPort{},
			Applications:    []remoteport.ListeningApplication{},
			NeedsPassphrase: true,
		}, nil
	}
	if err != nil {
		return HostPortDiscoveryResult{}, err
	}

	available := make(map[int][]string, len(snapshot.Ports))
	for _, port := range snapshot.Ports {
		available[port.Port] = append([]string(nil), port.Addresses...)
	}
	bindings.mu.Lock()
	bindings.passphrase = passphrase
	bindings.available = available
	bindings.mu.Unlock()

	if snapshot.Ports == nil {
		snapshot.Ports = []remoteport.ListeningPort{}
	}
	if snapshot.Applications == nil {
		snapshot.Applications = []remoteport.ListeningApplication{}
	}
	return HostPortDiscoveryResult{
		Metrics:      snapshot.Metrics,
		Ports:        snapshot.Ports,
		Applications: snapshot.Applications,
	}, nil
}

func (bindings *HostBindings) SavePortLink(link portlinkdomain.Link) (portlinkdomain.Link, error) {
	bindings.mu.Lock()
	link.ServerID = bindings.server.ID
	bindings.mu.Unlock()
	return bindings.links.Save(context.Background(), link)
}

func (bindings *HostBindings) DeletePortLink(id string) error {
	bindings.mu.Lock()
	serverID := bindings.server.ID
	bindings.mu.Unlock()
	items, err := bindings.links.ListByServer(context.Background(), serverID)
	if err != nil {
		return fmt.Errorf("load saved port links before deletion: %w", err)
	}
	found := false
	for _, item := range items {
		if item.ID == id {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("saved port link does not belong to this host")
	}
	return bindings.links.Delete(context.Background(), id)
}

func (bindings *HostBindings) OpenPort(port int, scheme string) (HostOpenPortResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), hostOpenTimeout)
	defer cancel()
	scheme, _, _, err := bindings.portAccess(port, scheme)
	if err != nil {
		return HostOpenPortResult{}, err
	}
	if bindings.preferences == nil || bindings.openProxyURL == nil {
		return HostOpenPortResult{}, fmt.Errorf("the SOCKS browser route is unavailable")
	}
	preference, err := bindings.preferences.Load(ctx)
	if err != nil {
		return HostOpenPortResult{}, fmt.Errorf("load proxy browser preference: %w", err)
	}
	target := &url.URL{
		Scheme: scheme,
		Host:   net.JoinHostPort("localhost", strconv.Itoa(port)),
	}
	if err := bindings.openProxyURL(
		ctx,
		configdomain.ManagedSOCKSConfigurationID(bindings.currentServerID()),
		browserIDForHostPort(preference, bindings.currentServerID(), port),
		target.String(),
	); err != nil {
		var protocolMismatch *control.ProtocolMismatchError
		if errors.As(err, &protocolMismatch) {
			return HostOpenPortResult{}, fmt.Errorf("restart SSH Man to use saved port links after this update")
		}
		return HostOpenPortResult{}, fmt.Errorf("open port through SOCKS browser: %w", err)
	}
	return HostOpenPortResult{URL: target.String()}, nil
}

func (bindings *HostBindings) currentServerID() string {
	bindings.mu.Lock()
	defer bindings.mu.Unlock()
	return bindings.server.ID
}

func browserIDForHostPort(preference preferencesdomain.UserPreference, serverID string, port int) string {
	for _, assignment := range preference.URLPortAssignments {
		if assignment.ServerID == serverID && assignment.Port == port {
			return assignment.BrowserID
		}
	}
	return preference.ProxyBrowserID
}

func (bindings *HostBindings) FindPortFavicon(port int, scheme string) (HostFaviconResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), hostOpenTimeout)
	defer cancel()
	scheme, passphrase, addresses, err := bindings.portAccess(port, scheme)
	if err != nil {
		return HostFaviconResult{}, err
	}
	bindings.mu.Lock()
	server := bindings.server
	bindings.mu.Unlock()
	target := &url.URL{
		Scheme: scheme,
		Host:   net.JoinHostPort(server.Host, strconv.Itoa(port)),
	}
	dialContext := func(dialCtx context.Context, _, _ string) (net.Conn, error) {
		return bindings.forwarder.DialRemote(dialCtx, passphrase, port, addresses)
	}
	dataURL, err := bindings.findFavicon(ctx, target.String(), dialContext)
	if err != nil {
		return HostFaviconResult{}, fmt.Errorf("find favicon for port %d: %w", port, err)
	}
	return HostFaviconResult{FaviconDataURL: dataURL}, nil
}

func (bindings *HostBindings) portAccess(port int, scheme string) (string, string, []string, error) {
	scheme = strings.ToLower(strings.TrimSpace(scheme))
	if scheme != string(portlinkdomain.SchemeHTTP) && scheme != string(portlinkdomain.SchemeHTTPS) {
		return "", "", nil, fmt.Errorf("port link scheme must be http or https")
	}

	bindings.mu.Lock()
	addresses, ok := bindings.available[port]
	addresses = append([]string(nil), addresses...)
	passphrase := bindings.passphrase
	bindings.mu.Unlock()
	if !ok {
		return "", "", nil, fmt.Errorf("refresh available ports before opening port %d", port)
	}
	return scheme, passphrase, addresses, nil
}

func hostServiceName(serverID string, port int) string {
	sum := sha256.Sum256([]byte(serverID + "\x00" + strconv.Itoa(port)))
	return "ssh-man-" + hex.EncodeToString(sum[:8]) + ".localhost"
}

func (bindings *HostBindings) Close() error {
	return bindings.window.Quit()
}

func (bindings *HostBindings) Shutdown(ctx context.Context) error {
	var forwarderErr error
	if bindings.forwarder != nil {
		forwarderErr = bindings.forwarder.Close()
	}
	var shutdownErr error
	if bindings.shutdown != nil {
		shutdownErr = bindings.shutdown(ctx)
	}
	return errors.Join(forwarderErr, shutdownErr)
}
