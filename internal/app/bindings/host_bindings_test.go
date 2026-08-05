package bindings

import (
	"context"
	"errors"
	"net"
	"reflect"
	"testing"

	appwindow "ssh-man/internal/app/window"
	portlinkdomain "ssh-man/internal/domain/portlink"
	preferencesdomain "ssh-man/internal/domain/preferences"
	serverdomain "ssh-man/internal/domain/server"
	"ssh-man/internal/ssh/auth"
	"ssh-man/internal/ssh/remoteport"
)

type fakePortDiscoverer struct {
	ports []remoteport.ListeningPort
	err   error
}

func (fake fakePortDiscoverer) Discover(context.Context, string) ([]remoteport.ListeningPort, error) {
	return fake.ports, fake.err
}

type recordingPortDiscoverer struct {
	passphrases []string
}

func (fake *recordingPortDiscoverer) Discover(_ context.Context, passphrase string) ([]remoteport.ListeningPort, error) {
	fake.passphrases = append(fake.passphrases, passphrase)
	return []remoteport.ListeningPort{{Port: 3000}}, nil
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
}

func (fake fakeHostPreferences) Load(context.Context) (preferencesdomain.UserPreference, error) {
	return fake.preference, nil
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

func TestHostBindingsDiscoversAndOpensOnlyCurrentPorts(t *testing.T) {
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
	if opened.URL != "http://ssh-man-test.localhost:43123" || forwarder.gotPort != 3000 {
		t.Fatalf("opened = %#v, forwarder = %#v", opened, forwarder)
	}
	if !reflect.DeepEqual(forwarder.gotAddresses, []string{"0.0.0.0", "127.0.0.1"}) {
		t.Fatalf("addresses = %#v", forwarder.gotAddresses)
	}
	if _, err := binding.OpenPort(8080, "http"); err == nil {
		t.Fatal("expected an undiscovered-port error")
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
