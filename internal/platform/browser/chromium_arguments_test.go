package browser

import (
	"reflect"
	"testing"
)

func TestChromiumProxyArguments(t *testing.T) {
	got := chromiumProxyArguments("localhost", "/tmp/chrome-profile", 41001)
	want := []string{
		"--proxy-server=socks5://localhost:41001",
		"--user-data-dir=/tmp/chrome-profile",
		"--proxy-bypass-list=<-loopback>",
		"--host-resolver-rules=MAP * ~NOTFOUND , EXCLUDE localhost",
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-search-engine-choice-screen",
		"--disable-sync",
		"--webrtc-ip-handling-policy=disable_non_proxied_udp",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("chromiumProxyArguments() = %#v, want %#v", got, want)
	}
}
