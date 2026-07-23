package auth

import (
	"errors"
	"testing"
)

func TestIsAuthenticationRejected(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "Go SSH handshake rejection",
			err:  errors.New("ssh: handshake failed: ssh: unable to authenticate, attempted methods [none publickey], no supported methods remain"),
			want: true,
		},
		{
			name: "OpenSSH public key rejection",
			err:  errors.New("eric@example.test: Permission denied (publickey,password)"),
			want: true,
		},
		{
			name: "network failure",
			err:  errors.New("connect to ssh server: dial tcp: connection refused"),
			want: false,
		},
		{
			name: "no error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsAuthenticationRejected(tt.err); got != tt.want {
				t.Fatalf("IsAuthenticationRejected() = %v, want %v", got, tt.want)
			}
		})
	}
}
