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
	portlinkdomain "ssh-man/internal/domain/portlink"
	preferencesdomain "ssh-man/internal/domain/preferences"
	serverdomain "ssh-man/internal/domain/server"
	"ssh-man/internal/ssh/auth"
	"ssh-man/internal/ssh/remoteport"
)

const hostOpenTimeout = 15 * time.Second

type portDiscoverer interface {
	Discover(context.Context, string) ([]remoteport.ListeningPort, error)
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

type HostInitialState struct {
	Server serverdomain.Server     `json:"server"`
	Links  []portlinkdomain.Link   `json:"links"`
	Theme  preferencesdomain.Theme `json:"theme"`
}

type HostPortDiscoveryResult struct {
	Ports           []remoteport.ListeningPort `json:"ports"`
	NeedsPassphrase bool                       `json:"needsPassphrase"`
}

type HostOpenPortResult struct {
	URL string `json:"url"`
}

type HostFaviconResult struct {
	FaviconDataURL string `json:"faviconDataUrl"`
}

type HostBindings struct {
	mu          sync.Mutex
	server      serverdomain.Server
	window      *appwindow.Controller
	links       portLinkService
	discoverer  portDiscoverer
	forwarder   portForwarder
	findFavicon faviconFinder
	preferences hostPreferenceService
	shutdown    func(context.Context) error
	passphrase  string
	available   map[int][]string
}

func NewHostBindings(app *bootstrap.Application, server serverdomain.Server, window *appwindow.Controller) *HostBindings {
	bindings := newHostBindingsWithDependencies(
		server,
		window,
		app.PortLinkService,
		remoteport.NewService(server),
		remoteport.NewForwarder(server),
		app.Shutdown,
	)
	bindings.preferences = app.PreferencesService
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
	items, err := bindings.links.ListByServer(context.Background(), bindings.server.ID)
	if err != nil {
		return HostInitialState{}, fmt.Errorf("load saved port links: %w", err)
	}
	if items == nil {
		items = []portlinkdomain.Link{}
	}
	theme := preferencesdomain.ThemeDark
	if bindings.preferences != nil {
		preference, preferenceErr := bindings.preferences.Load(context.Background())
		if preferenceErr != nil {
			return HostInitialState{}, fmt.Errorf("load host window theme: %w", preferenceErr)
		}
		theme = preference.Theme
	}
	return HostInitialState{Server: bindings.server, Links: items, Theme: theme}, nil
}

func (bindings *HostBindings) DiscoverPorts(passphrase string) (HostPortDiscoveryResult, error) {
	bindings.mu.Lock()
	if passphrase == "" {
		passphrase = bindings.passphrase
	}
	bindings.mu.Unlock()

	ports, err := bindings.discoverer.Discover(context.Background(), passphrase)
	if errors.Is(err, auth.ErrPassphraseRequired) {
		return HostPortDiscoveryResult{Ports: []remoteport.ListeningPort{}, NeedsPassphrase: true}, nil
	}
	if err != nil {
		return HostPortDiscoveryResult{}, err
	}

	available := make(map[int][]string, len(ports))
	for _, port := range ports {
		available[port.Port] = append([]string(nil), port.Addresses...)
	}
	bindings.mu.Lock()
	bindings.passphrase = passphrase
	bindings.available = available
	bindings.mu.Unlock()

	if ports == nil {
		ports = []remoteport.ListeningPort{}
	}
	return HostPortDiscoveryResult{Ports: ports}, nil
}

func (bindings *HostBindings) SavePortLink(link portlinkdomain.Link) (portlinkdomain.Link, error) {
	link.ServerID = bindings.server.ID
	return bindings.links.Save(context.Background(), link)
}

func (bindings *HostBindings) DeletePortLink(id string) error {
	items, err := bindings.links.ListByServer(context.Background(), bindings.server.ID)
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
	scheme, passphrase, addresses, err := bindings.portAccess(port, scheme)
	if err != nil {
		return HostOpenPortResult{}, err
	}
	forward, err := bindings.forwarder.Open(ctx, passphrase, port, addresses)
	if err != nil {
		return HostOpenPortResult{}, err
	}
	target := &url.URL{
		Scheme: scheme,
		Host:   net.JoinHostPort(forward.AccessHost, strconv.Itoa(forward.LocalPort)),
	}
	return HostOpenPortResult{URL: target.String()}, nil
}

func (bindings *HostBindings) FindPortFavicon(port int, scheme string) (HostFaviconResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), hostOpenTimeout)
	defer cancel()
	scheme, passphrase, addresses, err := bindings.portAccess(port, scheme)
	if err != nil {
		return HostFaviconResult{}, err
	}
	target := &url.URL{
		Scheme: scheme,
		Host:   net.JoinHostPort(hostServiceName(bindings.server.ID, port), strconv.Itoa(port)),
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
