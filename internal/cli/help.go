package cli

import "strings"

const rootUsage = `SSH Man controls saved SSH tunnels through the menu-bar agent.

Usage:
  ssh-man [global options] <command>

Commands:
  status                         List configured tunnels and live status
  diagnostics                    Show app data and control diagnostics
  app status|show|hide|quit      Manage the menu-bar agent
  server list|get|add|update|delete|start|stop
  tunnel list|get|add|update|delete|start|stop|restart|unlock|history
  browser list|preview|launch
  settings get|set
  version

Global options:
  -o, --output table|json|jsonl
      --connect-timeout DURATION
      --request-timeout DURATION
      --no-autostart
  -h, --help
      --version
`

const serverUsage = `Usage:
  ssh-man server list
  ssh-man server get SERVER
  ssh-man server add NAME --host HOST [--port 22] [--socks-port auto|PORT] [--user USER] [--auth agent|key] [--key PATH]
  ssh-man server update SERVER [--name NAME] [--host HOST] [--port PORT] [--socks-port auto|PORT] [--user USER] [--auth agent|key] [--key PATH]
  ssh-man server delete SERVER --yes [--stop-active]
  ssh-man server start SERVER
  ssh-man server stop SERVER
`

const tunnelUsage = `Usage:
  ssh-man tunnel list [--server SERVER] [--type local|socks] [--status STATUS] [--active]
  ssh-man tunnel get TUNNEL [--server SERVER]
  ssh-man tunnel add local LABEL --server SERVER --listen PORT --remote HOST:PORT [--reconnect=true|false] [--start-on-launch=true|false] [--notes TEXT]
  ssh-man tunnel add socks LABEL --server SERVER [--listen auto|PORT] [--reconnect=true|false] [--start-on-launch=true|false] [--notes TEXT]
  ssh-man tunnel update TUNNEL [--server SERVER] [--label LABEL] [--type local|socks] [--listen auto|PORT] [--remote HOST:PORT] [--reconnect=true|false] [--start-on-launch=true|false] [--notes TEXT|--clear-notes]
  ssh-man tunnel delete TUNNEL --yes [--server SERVER] [--stop-active]
  ssh-man tunnel start|stop|restart TUNNEL [--server SERVER]
  ssh-man tunnel unlock TUNNEL [--server SERVER] [--passphrase-stdin]
  ssh-man tunnel history TUNNEL [--server SERVER] [--limit N]
`

const appUsage = `Usage:
  ssh-man app status
  ssh-man app show
  ssh-man app hide
  ssh-man app quit [--yes]
`

const browserUsage = `Usage:
  ssh-man browser list
  ssh-man browser preview --tunnel TUNNEL --browser BROWSER [--server SERVER]
  ssh-man browser launch --tunnel TUNNEL --browser BROWSER [--server SERVER]
`

const settingsUsage = `Usage:
  ssh-man settings get
  ssh-man settings set --theme dark|light
`

func usageFor(args []string) string {
	words := make([]string, 0, len(args))
	for _, argument := range args {
		if !strings.HasPrefix(argument, "-") {
			words = append(words, argument)
		}
	}
	if len(words) > 0 && words[0] == "help" {
		words = words[1:]
	}
	if len(words) == 0 {
		return rootUsage
	}
	switch words[0] {
	case "app":
		return appUsage
	case "server":
		return serverUsage
	case "tunnel":
		return tunnelUsage
	case "browser":
		return browserUsage
	case "settings":
		return settingsUsage
	default:
		return rootUsage
	}
}
