package bindings

import (
	"context"
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

type portForwarder interface {
	Open(context.Context, string, int, []string) (remoteport.Forward, error)
	Close() error
}

type faviconFinder func(context.Context, string) (string, error)

type HostInitialState struct {
	Server serverdomain.Server   `json:"server"`
	Links  []portlinkdomain.Link `json:"links"`
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
	shutdown    func(context.Context) error
	passphrase  string
	available   map[int][]string
}

func NewHostBindings(app *bootstrap.Application, server serverdomain.Server, window *appwindow.Controller) *HostBindings {
	return newHostBindingsWithDependencies(
		server,
		window,
		app.PortLinkService,
		remoteport.NewService(server),
		remoteport.NewForwarder(server),
		app.Shutdown,
	)
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
	findFavicon := remoteport.FindFavicon
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
	return HostInitialState{Server: bindings.server, Links: items}, nil
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
	target, err := bindings.openPortURL(ctx, port, scheme)
	if err != nil {
		return HostOpenPortResult{}, err
	}
	return HostOpenPortResult{URL: target.String()}, nil
}

func (bindings *HostBindings) FindPortFavicon(port int, scheme string) (HostFaviconResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), hostOpenTimeout)
	defer cancel()
	target, err := bindings.openPortURL(ctx, port, scheme)
	if err != nil {
		return HostFaviconResult{}, err
	}
	dataURL, err := bindings.findFavicon(ctx, target.String())
	if err != nil {
		return HostFaviconResult{}, fmt.Errorf("find favicon for port %d: %w", port, err)
	}
	return HostFaviconResult{FaviconDataURL: dataURL}, nil
}

func (bindings *HostBindings) openPortURL(ctx context.Context, port int, scheme string) (*url.URL, error) {
	scheme = strings.ToLower(strings.TrimSpace(scheme))
	if scheme != string(portlinkdomain.SchemeHTTP) && scheme != string(portlinkdomain.SchemeHTTPS) {
		return nil, fmt.Errorf("port link scheme must be http or https")
	}

	bindings.mu.Lock()
	addresses, ok := bindings.available[port]
	addresses = append([]string(nil), addresses...)
	passphrase := bindings.passphrase
	bindings.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("refresh available ports before opening port %d", port)
	}

	forward, err := bindings.forwarder.Open(ctx, passphrase, port, addresses)
	if err != nil {
		return nil, err
	}
	return &url.URL{
		Scheme: scheme,
		Host:   net.JoinHostPort("127.0.0.1", strconv.Itoa(forward.LocalPort)),
	}, nil
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
