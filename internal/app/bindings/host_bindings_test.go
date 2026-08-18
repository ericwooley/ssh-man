package bindings

import (
	"context"
	"errors"
	"net"
	"reflect"
	"strings"
	"testing"

	appwindow "ssh-man/internal/app/window"
	"ssh-man/internal/control"
	configdomain "ssh-man/internal/domain/config"
	portlinkdomain "ssh-man/internal/domain/portlink"
	preferencesdomain "ssh-man/internal/domain/preferences"
	serverdomain "ssh-man/internal/domain/server"
	sessiondomain "ssh-man/internal/domain/session"
	"ssh-man/internal/ssh/auth"
	"ssh-man/internal/ssh/remoteport"
)

type fakePortDiscoverer struct {
	ports        []remoteport.ListeningPort
	metrics      remoteport.HostMetrics
	applications []remoteport.ListeningApplication
	err          error
}

func (fake fakePortDiscoverer) Discover(context.Context, string) (remoteport.DashboardSnapshot, error) {
	return remoteport.DashboardSnapshot{
		Metrics:      fake.metrics,
		Ports:        fake.ports,
		Applications: fake.applications,
	}, fake.err
}

type recordingPortDiscoverer struct {
	passphrases []string
}

func (fake *recordingPortDiscoverer) Discover(_ context.Context, passphrase string) (remoteport.DashboardSnapshot, error) {
	fake.passphrases = append(fake.passphrases, passphrase)
	return remoteport.DashboardSnapshot{Ports: []remoteport.ListeningPort{{Port: 3000}}}, nil
}

type fakePortLinkService struct {
	items     []portlinkdomain.Link
	saved     portlinkdomain.Link
	deletedID string
}

func (fake *fakePortLinkService) ListByServer(context.Context, string) ([]portlinkdomain.Link, error) {
	return fake.items, nil
}

func (fake *fakePortLinkService) Save(_ context.Context, link portlinkdomain.Link) (portlinkdomain.Link, error) {
	fake.saved = link
	return link, nil
}

func (fake *fakePortLinkService) Delete(_ context.Context, id string) error {
	fake.deletedID = id
	return nil
}

type fakePortForwarder struct {
	gotPort      int
	gotAddresses []string
	openCalls    int
	directCalls  int
}

type fakeHostPreferences struct {
	preference preferencesdomain.UserPreference
	err        error
}

type fakeHostControlClient struct {
	call func(context.Context, control.Request, any) error
}

func (fake fakeHostControlClient) Call(ctx context.Context, request control.Request, output any) error {
	return fake.call(ctx, request, output)
}

func (fake fakeHostPreferences) Load(context.Context) (preferencesdomain.UserPreference, error) {
	return fake.preference, fake.err
}

func (fake *fakePortForwarder) Open(_ context.Context, _ string, port int, addresses []string) (remoteport.Forward, error) {
	fake.openCalls++
	fake.gotPort = port
	fake.gotAddresses = addresses
	return remoteport.Forward{
		RemotePort: port,
		LocalPort:  43123,
		RemoteHost: "127.0.0.1",
		AccessHost: "ssh-man-test.localhost",
	}, nil
}

func (fake *fakePortForwarder) DialRemote(
	_ context.Context,
	_ string,
	port int,
	addresses []string,
) (net.Conn, error) {
	fake.directCalls++
	fake.gotPort = port
	fake.gotAddresses = addresses
	client, server := net.Pipe()
	_ = server.Close()
	return client, nil
}

func (*fakePortForwarder) Close() error {
	return nil
}

func TestHostBindingsOpensCurrentPortThroughAssignedServerProxyBrowser(t *testing.T) {
	server := serverdomain.Server{ID: "server-1", Name: "Production"}
	forwarder := &fakePortForwarder{}
	binding := newHostBindingsWithDependencies(
		server,
		appwindow.New(),
		&fakePortLinkService{},
		fakePortDiscoverer{ports: []remoteport.ListeningPort{{
			Port:            3000,
			Addresses:       []string{"0.0.0.0", "127.0.0.1"},
			SuggestedScheme: remoteport.SchemeHTTP,
		}}},
		forwarder,
		nil,
	)
	binding.preferences = fakeHostPreferences{preference: preferencesdomain.UserPreference{
		ProxyBrowserID: "google-chrome",
		URLPortAssignments: []preferencesdomain.URLPortAssignment{{
			Port:      3000,
			ServerID:  "server-1",
			BrowserID: "firefox",
		}},
	}}
	var gotConfigurationID, gotBrowserID, gotURL string
	binding.openProxyURL = func(_ context.Context, configurationID, browserID, rawURL string) error {
		gotConfigurationID = configurationID
		gotBrowserID = browserID
		gotURL = rawURL
		return nil
	}

	result, err := binding.DiscoverPorts("secret")
	if err != nil {
		t.Fatal(err)
	}
	if result.NeedsPassphrase || len(result.Ports) != 1 {
		t.Fatalf("discovery = %#v", result)
	}
	opened, err := binding.OpenPort(3000, "http")
	if err != nil {
		t.Fatal(err)
	}
	if opened.URL != "http://localhost:3000" {
		t.Fatalf("opened = %#v", opened)
	}
	if gotConfigurationID != configdomain.ManagedSOCKSConfigurationID("server-1") ||
		gotBrowserID != "firefox" ||
		gotURL != "http://localhost:3000" {
		t.Fatalf("proxy launch = configuration %q, browser %q, URL %q", gotConfigurationID, gotBrowserID, gotURL)
	}
	if forwarder.openCalls != 0 {
		t.Fatal("opening a port created a direct local forward")
	}
	if _, err := binding.OpenPort(8080, "http"); err == nil {
		t.Fatal("expected an undiscovered-port error")
	}
}

func TestBrowserIDForHostPortUsesProxyDefaultWithoutMatchingAssignment(t *testing.T) {
	preference := preferencesdomain.UserPreference{
		ProxyBrowserID: "google-chrome",
		URLPortAssignments: []preferencesdomain.URLPortAssignment{{
			Port:      3000,
			ServerID:  "another-server",
			BrowserID: "firefox",
		}},
	}
	if got := browserIDForHostPort(preference, "server-1", 3000); got != "google-chrome" {
		t.Fatalf("browser id = %q, want google-chrome", got)
	}
}

func TestHostBindingsRequestsRestartForControlProtocolMismatch(t *testing.T) {
	binding := newHostBindingsWithDependencies(
		serverdomain.Server{ID: "server-1"},
		appwindow.New(),
		&fakePortLinkService{},
		fakePortDiscoverer{ports: []remoteport.ListeningPort{{Port: 4321}}},
		&fakePortForwarder{},
		nil,
	)
	binding.preferences = fakeHostPreferences{preference: preferencesdomain.UserPreference{
		ProxyBrowserID: "firefox",
	}}
	binding.openProxyURL = func(context.Context, string, string, string) error {
		return &control.ProtocolMismatchError{AppVersion: 3, CLIVersion: 4}
	}
	if _, err := binding.DiscoverPorts(""); err != nil {
		t.Fatal(err)
	}

	_, err := binding.OpenPort(4321, "http")
	if err == nil || !strings.Contains(err.Error(), "restart SSH Man") {
		t.Fatalf("OpenPort() error = %v, want restart guidance", err)
	}
}

func TestHostBindingsFindsFaviconThroughCurrentPort(t *testing.T) {
	var gotURL string
	binding := newHostBindingsWithDependencies(
		serverdomain.Server{ID: "server-1", Name: "Production", Host: "prod.example.com"},
		appwindow.New(),
		&fakePortLinkService{},
		fakePortDiscoverer{ports: []remoteport.ListeningPort{{
			Port:      3000,
			Addresses: []string{"127.0.0.1"},
		}}},
		&fakePortForwarder{},
		nil,
		func(ctx context.Context, rawURL string, dial remoteport.DialContextFunc) (string, error) {
			gotURL = rawURL
			connection, dialErr := dial(ctx, "tcp", "ignored")
			if dialErr != nil {
				return "", dialErr
			}
			_ = connection.Close()
			return "data:image/png;base64,aWNvbg==", nil
		},
	)
	if _, err := binding.DiscoverPorts(""); err != nil {
		t.Fatal(err)
	}

	result, err := binding.FindPortFavicon(3000, "http")
	if err != nil {
		t.Fatal(err)
	}
	if gotURL != "http://prod.example.com:3000" {
		t.Fatalf("favicon URL = %q", gotURL)
	}
	if result.FaviconDataURL != "data:image/png;base64,aWNvbg==" {
		t.Fatalf("favicon result = %#v", result)
	}
	if binding.forwarder.(*fakePortForwarder).openCalls != 0 {
		t.Fatal("favicon lookup opened a persistent local forward")
	}
	if binding.forwarder.(*fakePortForwarder).directCalls != 1 {
		t.Fatal("favicon lookup did not use one direct remote connection")
	}
	if _, err := binding.FindPortFavicon(8080, "http"); err == nil {
		t.Fatal("expected an undiscovered-port error")
	}
}

func TestHostBindingsReturnsPassphraseState(t *testing.T) {
	binding := newHostBindingsWithDependencies(
		serverdomain.Server{ID: "server-1"},
		appwindow.New(),
		&fakePortLinkService{},
		fakePortDiscoverer{err: auth.ErrPassphraseRequired},
		&fakePortForwarder{},
		nil,
	)

	result, err := binding.DiscoverPorts("")
	if err != nil {
		t.Fatal(err)
	}
	if !result.NeedsPassphrase {
		t.Fatalf("discovery = %#v", result)
	}
}

func TestHostBindingsReturnsDashboardData(t *testing.T) {
	binding := newHostBindingsWithDependencies(
		serverdomain.Server{ID: "server-1"},
		appwindow.New(),
		&fakePortLinkService{},
		fakePortDiscoverer{
			ports:        []remoteport.ListeningPort{{Port: 3000}},
			metrics:      remoteport.HostMetrics{MemoryTotalBytes: 1024, CPUCount: 4},
			applications: []remoteport.ListeningApplication{{Name: "node", PID: 12, Ports: []int{3000}}},
		},
		&fakePortForwarder{},
		nil,
	)

	result, err := binding.DiscoverPorts("")
	if err != nil {
		t.Fatal(err)
	}
	if result.Metrics.MemoryTotalBytes != 1024 || result.Metrics.CPUCount != 4 {
		t.Fatalf("metrics = %#v", result.Metrics)
	}
	if len(result.Applications) != 1 || result.Applications[0].Name != "node" {
		t.Fatalf("applications = %#v", result.Applications)
	}
}

func TestHostBindingsScopesSavedLinksToItsServer(t *testing.T) {
	links := &fakePortLinkService{}
	binding := newHostBindingsWithDependencies(
		serverdomain.Server{ID: "server-1"},
		appwindow.New(),
		links,
		fakePortDiscoverer{},
		&fakePortForwarder{},
		nil,
	)

	saved, err := binding.SavePortLink(portlinkdomain.Link{
		ServerID: "another-server",
		Port:     8080,
		Name:     "Admin",
		Scheme:   portlinkdomain.SchemeHTTP,
	})
	if err != nil {
		t.Fatal(err)
	}
	if saved.ServerID != "server-1" || links.saved.ServerID != "server-1" {
		t.Fatalf("saved link = %#v", saved)
	}
}

func TestHostBindingsRejectsUnsupportedScheme(t *testing.T) {
	binding := newHostBindingsWithDependencies(
		serverdomain.Server{ID: "server-1"},
		appwindow.New(),
		&fakePortLinkService{},
		fakePortDiscoverer{ports: []remoteport.ListeningPort{{Port: 3000}}},
		&fakePortForwarder{},
		nil,
	)
	if _, err := binding.DiscoverPorts(""); err != nil {
		t.Fatal(err)
	}
	if _, err := binding.OpenPort(3000, "file"); err == nil {
		t.Fatal("expected unsupported scheme error")
	}
}

func TestHostBindingsInitialStateIncludesSavedLinks(t *testing.T) {
	want := portlinkdomain.Link{ID: "link-1", ServerID: "server-1", Port: 3000}
	binding := newHostBindingsWithDependencies(
		serverdomain.Server{ID: "server-1"},
		appwindow.New(),
		&fakePortLinkService{items: []portlinkdomain.Link{want}},
		fakePortDiscoverer{},
		&fakePortForwarder{},
		func(context.Context) error { return errors.New("unused") },
	)
	state, err := binding.InitialState()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Links) != 1 || state.Links[0].ID != want.ID {
		t.Fatalf("initial state = %#v", state)
	}
}

func TestHostBindingsInitialStateIncludesSavedTheme(t *testing.T) {
	for _, theme := range []preferencesdomain.Theme{preferencesdomain.ThemeLight, preferencesdomain.ThemeDark} {
		t.Run(string(theme), func(t *testing.T) {
			binding := newHostBindingsWithDependencies(
				serverdomain.Server{ID: "server-1"},
				appwindow.New(),
				&fakePortLinkService{},
				fakePortDiscoverer{},
				&fakePortForwarder{},
				nil,
			)
			binding.preferences = fakeHostPreferences{preference: preferencesdomain.UserPreference{Theme: theme}}

			state, err := binding.InitialState()
			if err != nil {
				t.Fatal(err)
			}
			if state.Theme != theme {
				t.Fatalf("theme = %q, want %q", state.Theme, theme)
			}
		})
	}
}

func TestHostBindingsInitialStateUsesDefaultThemeWhenPreferencesFail(t *testing.T) {
	want := portlinkdomain.Link{ID: "link-1", ServerID: "server-1", Port: 3000}
	binding := newHostBindingsWithDependencies(
		serverdomain.Server{ID: "server-1"},
		appwindow.New(),
		&fakePortLinkService{items: []portlinkdomain.Link{want}},
		fakePortDiscoverer{},
		&fakePortForwarder{},
		nil,
	)
	binding.preferences = fakeHostPreferences{err: errors.New("preferences unavailable")}

	state, err := binding.InitialState()
	if err != nil {
		t.Fatal(err)
	}
	if state.Theme != preferencesdomain.ThemeDark {
		t.Fatalf("theme = %q, want %q", state.Theme, preferencesdomain.ThemeDark)
	}
	if state.Server.ID != "server-1" || len(state.Links) != 1 || state.Links[0].ID != want.ID {
		t.Fatalf("initial state = %#v", state)
	}
}

func TestHostBindingsReusesUnlockedPassphraseForRefresh(t *testing.T) {
	discoverer := &recordingPortDiscoverer{}
	binding := newHostBindingsWithDependencies(
		serverdomain.Server{ID: "server-1"},
		appwindow.New(),
		&fakePortLinkService{},
		discoverer,
		&fakePortForwarder{},
		nil,
	)

	if _, err := binding.DiscoverPorts("secret"); err != nil {
		t.Fatal(err)
	}
	if _, err := binding.DiscoverPorts(""); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(discoverer.passphrases, []string{"secret", "secret"}) {
		t.Fatalf("discovery passphrases = %#v", discoverer.passphrases)
	}
}

func TestHostBindingsDoesNotDeleteAnotherServersLink(t *testing.T) {
	links := &fakePortLinkService{items: []portlinkdomain.Link{{
		ID:       "link-1",
		ServerID: "server-1",
		Port:     3000,
	}}}
	binding := newHostBindingsWithDependencies(
		serverdomain.Server{ID: "server-1"},
		appwindow.New(),
		links,
		fakePortDiscoverer{},
		&fakePortForwarder{},
		nil,
	)

	if err := binding.DeletePortLink("another-link"); err == nil {
		t.Fatal("expected a scoped-link error")
	}
	if links.deletedID != "" {
		t.Fatalf("deleted link = %q", links.deletedID)
	}
	if err := binding.DeletePortLink("link-1"); err != nil {
		t.Fatal(err)
	}
	if links.deletedID != "link-1" {
		t.Fatalf("deleted link = %q", links.deletedID)
	}
}

func TestHostBindingsLoadsControllerStateForOnlyItsServer(t *testing.T) {
	binding := newHostBindingsWithDependencies(
		serverdomain.Server{ID: "server-1"},
		appwindow.New(),
		&fakePortLinkService{},
		fakePortDiscoverer{},
		&fakePortForwarder{},
		nil,
	)
	binding.control = fakeHostControlClient{call: func(_ context.Context, request control.Request, output any) error {
		if request.Command != "state" {
			t.Fatalf("control command = %q, want state", request.Command)
		}
		state := output.(*control.State)
		*state = control.State{
			Servers: []control.ServerRecord{
				{
					Server: serverdomain.Server{ID: "server-1", Name: "Production", SocksPort: 41000},
					Configurations: []configdomain.ConnectionConfiguration{{
						ID: "tunnel-1", ServerID: "server-1", Label: "Admin",
					}},
				},
				{
					Server: serverdomain.Server{ID: "server-2", Name: "Staging"},
					Configurations: []configdomain.ConnectionConfiguration{{
						ID: "tunnel-2", ServerID: "server-2", Label: "Preview",
					}},
				},
			},
			Preferences: preferencesdomain.UserPreference{Theme: preferencesdomain.ThemeLight},
			Sessions: []sessiondomain.RuntimeSession{
				{ConfigurationID: "tunnel-1", Status: sessiondomain.StatusConnected},
				{ConfigurationID: configdomain.ManagedSOCKSConfigurationID("server-1"), Status: sessiondomain.StatusConnected},
				{ConfigurationID: "tunnel-2", Status: sessiondomain.StatusConnected},
			},
		}
		return nil
	}}

	state, err := binding.LoadAppState()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Servers) != 1 || state.Servers[0].Server.ID != "server-1" {
		t.Fatalf("servers = %#v, want only server-1", state.Servers)
	}
	configurations := state.Servers[0].Configurations
	if len(configurations) != 2 ||
		configurations[0].ID != configdomain.ManagedSOCKSConfigurationID("server-1") ||
		configurations[1].ID != "tunnel-1" {
		t.Fatalf("configurations = %#v", configurations)
	}
	if len(state.Sessions) != 2 {
		t.Fatalf("sessions = %#v, want only server-1 sessions", state.Sessions)
	}
	if state.Preferences.Theme != preferencesdomain.ThemeLight {
		t.Fatalf("theme = %q, want light", state.Preferences.Theme)
	}
}

func TestHostBindingsScopesTunnelActionsToItsServer(t *testing.T) {
	var actionRequests []control.Request
	binding := newHostBindingsWithDependencies(
		serverdomain.Server{ID: "server-1"},
		appwindow.New(),
		&fakePortLinkService{},
		fakePortDiscoverer{},
		&fakePortForwarder{},
		nil,
	)
	binding.control = fakeHostControlClient{call: func(_ context.Context, request control.Request, output any) error {
		switch request.Command {
		case "state":
			state := output.(*control.State)
			*state = control.State{Servers: []control.ServerRecord{{
				Server: serverdomain.Server{ID: "server-1"},
				Configurations: []configdomain.ConnectionConfiguration{{
					ID: "tunnel-1", ServerID: "server-1", Label: "Admin",
				}},
			}}}
		case "session.start":
			actionRequests = append(actionRequests, request)
			session := output.(*sessiondomain.RuntimeSession)
			*session = sessiondomain.RuntimeSession{
				ConfigurationID: request.ConfigurationID,
				Status:          sessiondomain.StatusConnected,
			}
		default:
			t.Fatalf("unexpected control command %q", request.Command)
		}
		return nil
	}}

	session, err := binding.StartConfiguration("tunnel-1")
	if err != nil {
		t.Fatal(err)
	}
	if session.ConfigurationID != "tunnel-1" || len(actionRequests) != 1 {
		t.Fatalf("session = %#v, action requests = %#v", session, actionRequests)
	}
	if _, err := binding.StartConfiguration("tunnel-2"); err == nil {
		t.Fatal("expected another server's tunnel to be rejected")
	}
	if len(actionRequests) != 1 {
		t.Fatalf("action requests = %#v, want no unscoped action", actionRequests)
	}
}
