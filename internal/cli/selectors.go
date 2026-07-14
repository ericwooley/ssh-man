package cli

import (
	"fmt"
	"sort"
	"strings"

	"ssh-man/internal/control"
	configdomain "ssh-man/internal/domain/config"
	serverdomain "ssh-man/internal/domain/server"
	sessiondomain "ssh-man/internal/domain/session"
)

type selectorError struct{ message string }

func (e *selectorError) Error() string { return e.message }

type tunnelRecord struct {
	Server        serverdomain.Server
	Configuration configdomain.ConnectionConfiguration
	Session       sessiondomain.RuntimeSession
}

func resolveServer(state control.State, selector string) (control.ServerRecord, error) {
	selector = strings.TrimSpace(selector)
	for _, record := range state.Servers {
		if record.Server.ID == selector {
			return record, nil
		}
	}
	matches := make([]control.ServerRecord, 0, 1)
	for _, record := range state.Servers {
		if strings.EqualFold(record.Server.Name, selector) {
			matches = append(matches, record)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		candidates := make([]string, 0, len(matches))
		for _, match := range matches {
			candidates = append(candidates, fmt.Sprintf("%s (%s)", match.Server.Name, match.Server.ID))
		}
		sort.Strings(candidates)
		return control.ServerRecord{}, &selectorError{message: fmt.Sprintf("server %q is ambiguous; use an ID: %s", selector, strings.Join(candidates, ", "))}
	}
	return control.ServerRecord{}, &selectorError{message: fmt.Sprintf("server %q was not found", selector)}
}

func resolveTunnel(state control.State, selector string, serverSelector string) (tunnelRecord, error) {
	serverID := ""
	if strings.TrimSpace(serverSelector) != "" {
		record, err := resolveServer(state, serverSelector)
		if err != nil {
			return tunnelRecord{}, err
		}
		serverID = record.Server.ID
	}

	all := tunnelRecords(state)
	for _, record := range all {
		if record.Configuration.ID == selector && (serverID == "" || record.Server.ID == serverID) {
			return record, nil
		}
	}

	matches := make([]tunnelRecord, 0)
	for _, record := range all {
		if strings.EqualFold(record.Configuration.Label, selector) && (serverID == "" || record.Server.ID == serverID) {
			matches = append(matches, record)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) == 0 {
		return tunnelRecord{}, &selectorError{message: fmt.Sprintf("tunnel %q was not found", selector)}
	}

	candidates := make([]string, 0, len(matches))
	for _, match := range matches {
		candidates = append(candidates, fmt.Sprintf("%s/%s (%s)", match.Server.Name, match.Configuration.Label, match.Configuration.ID))
	}
	sort.Strings(candidates)
	return tunnelRecord{}, &selectorError{message: fmt.Sprintf("tunnel %q is ambiguous; use --server or an ID: %s", selector, strings.Join(candidates, ", "))}
}

func tunnelRecords(state control.State) []tunnelRecord {
	sessions := make(map[string]sessiondomain.RuntimeSession, len(state.Sessions))
	for _, session := range state.Sessions {
		sessions[session.ConfigurationID] = session
	}

	records := make([]tunnelRecord, 0)
	for _, server := range state.Servers {
		for _, configuration := range server.Configurations {
			session, ok := sessions[configuration.ID]
			if !ok {
				session = sessiondomain.RuntimeSession{ConfigurationID: configuration.ID, Status: sessiondomain.StatusStopped}
			}
			records = append(records, tunnelRecord{Server: server.Server, Configuration: configuration, Session: session})
		}
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].Server.Name != records[j].Server.Name {
			return records[i].Server.Name < records[j].Server.Name
		}
		if records[i].Configuration.Label != records[j].Configuration.Label {
			return records[i].Configuration.Label < records[j].Configuration.Label
		}
		return records[i].Configuration.ID < records[j].Configuration.ID
	})
	return records
}

func isActive(status sessiondomain.Status) bool {
	switch status {
	case sessiondomain.StatusStarting, sessiondomain.StatusConnected, sessiondomain.StatusReconnecting, sessiondomain.StatusNeedsAttention:
		return true
	default:
		return false
	}
}
