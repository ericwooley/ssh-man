package browser

import "fmt"

func chromiumProxyArguments(proxyHost string, profileDir string, socksPort int) []string {
	return []string{
		fmt.Sprintf("--proxy-server=socks5://%s:%d", proxyHost, socksPort),
		fmt.Sprintf("--user-data-dir=%s", profileDir),
		"--proxy-bypass-list=<-loopback>",
		fmt.Sprintf("--host-resolver-rules=MAP * ~NOTFOUND , EXCLUDE %s", proxyHost),
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-search-engine-choice-screen",
		"--disable-sync",
		"--webrtc-ip-handling-policy=disable_non_proxied_udp",
	}
}
