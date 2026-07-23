package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"ssh-man/internal/control"
	"ssh-man/internal/domain/config"
	"ssh-man/internal/domain/server"
	"ssh-man/internal/domain/session"
	"ssh-man/internal/platform/browser"
)

type table struct {
	Header []string
	Rows   [][]string
}

func writeOutput(writer io.Writer, mode string, value any, human table) error {
	switch mode {
	case "json":
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(value)
	case "jsonl":
		return writeJSONLines(writer, value)
	default:
		return writeTable(writer, human)
	}
}

func writeJSONLines(writer io.Writer, value any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	var items []json.RawMessage
	if len(encoded) > 0 && encoded[0] == '[' {
		if err := json.Unmarshal(encoded, &items); err != nil {
			return err
		}
	} else {
		items = []json.RawMessage{encoded}
	}
	for _, item := range items {
		if _, err := fmt.Fprintln(writer, string(item)); err != nil {
			return err
		}
	}
	return nil
}

func writeTable(writer io.Writer, value table) error {
	tab := tabwriter.NewWriter(writer, 0, 4, 2, ' ', 0)
	if len(value.Header) > 0 {
		if _, err := fmt.Fprintln(tab, strings.Join(value.Header, "\t")); err != nil {
			return err
		}
	}
	for _, row := range value.Rows {
		if _, err := fmt.Fprintln(tab, strings.Join(row, "\t")); err != nil {
			return err
		}
	}
	return tab.Flush()
}

func serverTable(state control.State, records []control.ServerRecord) table {
	rows := make([][]string, 0, len(records))
	for _, record := range records {
		active := 0
		browserSOCKSPort := "auto"
		if record.Server.SocksPort > 0 {
			browserSOCKSPort = strconv.Itoa(record.Server.SocksPort)
		}
		for _, configuration := range record.Configurations {
			for _, runtime := range state.Sessions {
				if runtime.ConfigurationID == configuration.ID && isActive(runtime.Status) {
					active++
				}
			}
		}
		rows = append(rows, []string{
			record.Server.ID,
			record.Server.Name,
			fmt.Sprintf("%s@%s:%d", record.Server.Username, record.Server.Host, record.Server.Port),
			string(record.Server.AuthMode),
			browserSOCKSPort,
			strconv.Itoa(len(record.Configurations)),
			strconv.Itoa(active),
		})
	}
	return table{Header: []string{"ID", "NAME", "DESTINATION", "AUTH", "BROWSER SOCKS", "TUNNELS", "ACTIVE"}, Rows: rows}
}

func tunnelTable(records []tunnelRecord) table {
	rows := make([][]string, 0, len(records))
	for _, record := range records {
		listen := ""
		target := "SOCKS5"
		if record.Configuration.ConnectionType == config.ConnectionTypeSOCKSProxy {
			if record.Session.BoundPort > 0 {
				listen = strconv.Itoa(record.Session.BoundPort)
			} else if record.Configuration.SocksPort == 0 {
				listen = "auto"
			} else {
				listen = strconv.Itoa(record.Configuration.SocksPort)
			}
		} else {
			listen = strconv.Itoa(record.Configuration.LocalPort)
			target = fmt.Sprintf("%s:%d", record.Configuration.RemoteHost, record.Configuration.RemotePort)
		}
		rows = append(rows, []string{
			record.Configuration.ID,
			record.Server.Name,
			record.Configuration.Label,
			string(record.Configuration.ConnectionType),
			listen,
			target,
			string(record.Session.Status),
		})
	}
	return table{Header: []string{"ID", "SERVER", "LABEL", "TYPE", "LISTEN", "TARGET", "STATUS"}, Rows: rows}
}

func sessionTable(states []session.RuntimeSession) table {
	rows := make([][]string, 0, len(states))
	for _, state := range states {
		rows = append(rows, []string{state.ConfigurationID, string(state.Status), strconv.Itoa(state.BoundPort), state.StatusDetail})
	}
	return table{Header: []string{"TUNNEL_ID", "STATUS", "BOUND_PORT", "DETAIL"}, Rows: rows}
}

func historyTable(entries []session.SessionHistoryEntry) table {
	rows := make([][]string, 0, len(entries))
	for _, entry := range entries {
		rows = append(rows, []string{entry.StartedAt.UTC().Format("2006-01-02T15:04:05Z"), entry.EndedAt.UTC().Format("2006-01-02T15:04:05Z"), string(entry.Outcome), entry.Message})
	}
	return table{Header: []string{"STARTED", "ENDED", "OUTCOME", "MESSAGE"}, Rows: rows}
}

func browserTable(items []browser.BrowserOption) table {
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		rows = append(rows, []string{item.ID, item.DisplayName, strconv.FormatBool(item.SupportsProxyLaunch)})
	}
	return table{Header: []string{"ID", "NAME", "PROXY_LAUNCH"}, Rows: rows}
}

func serverJSON(records []control.ServerRecord) any {
	if records == nil {
		return []control.ServerRecord{}
	}
	return records
}

type tunnelOutput struct {
	Server        server.Server                  `json:"server"`
	Configuration config.ConnectionConfiguration `json:"configuration"`
	Session       session.RuntimeSession         `json:"session"`
}

func tunnelJSON(records []tunnelRecord) []tunnelOutput {
	result := make([]tunnelOutput, 0, len(records))
	for _, record := range records {
		result = append(result, tunnelOutput{Server: record.Server, Configuration: record.Configuration, Session: record.Session})
	}
	return result
}

func singleTunnelJSON(record tunnelRecord) tunnelOutput {
	return tunnelOutput{Server: record.Server, Configuration: record.Configuration, Session: record.Session}
}

func sortedServerRecords(state control.State) []control.ServerRecord {
	records := append([]control.ServerRecord(nil), state.Servers...)
	sort.Slice(records, func(i, j int) bool {
		if records[i].Server.Name != records[j].Server.Name {
			return records[i].Server.Name < records[j].Server.Name
		}
		return records[i].Server.ID < records[j].Server.ID
	})
	return records
}
