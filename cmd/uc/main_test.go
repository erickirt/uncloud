package main

import (
	"slices"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

func TestApplyGlobalFlagsFromCompletionArgs(t *testing.T) {
	defaultConfigPath := "~/.config/uncloud/config.yaml"

	tests := []struct {
		name           string
		args           []string
		wantConnect    string
		wantContext    string
		wantConfigPath string
		// Flag names expected to be marked as changed on the target flag set.
		wantChanged []string
	}{
		{
			name: "no flags",
			args: []string{"__complete", "inspect", ""},
		},
		{
			name:        "connect with space",
			args:        []string{"__complete", "--connect", "ssh://user@host", "inspect", ""},
			wantConnect: "ssh://user@host",
			wantChanged: []string{"connect"},
		},
		{
			name:        "connect with equals",
			args:        []string{"__complete", "--connect=tcp://127.0.0.1:51000", "inspect", ""},
			wantConnect: "tcp://127.0.0.1:51000",
			wantChanged: []string{"connect"},
		},
		{
			name:        "context shorthand",
			args:        []string{"__complete", "-c", "prod", "inspect", ""},
			wantContext: "prod",
			wantChanged: []string{"context"},
		},
		{
			name:           "all flags",
			args:           []string{"__complete", "--connect", "user@host", "-c", "prod", "--uncloud-config", "/tmp/uncloud.yaml", "inspect", ""},
			wantConnect:    "user@host",
			wantContext:    "prod",
			wantConfigPath: "/tmp/uncloud.yaml",
			wantChanged:    []string{"connect", "context", "uncloud-config"},
		},
		{
			name:        "unknown flags are ignored",
			args:        []string{"__complete", "--quiet", "-n", "5", "--connect", "user@host", "logs", ""},
			wantConnect: "user@host",
			wantChanged: []string{"connect"},
		},
		{
			name: "flags after double dash are ignored",
			args: []string{"__complete", "exec", "svc", "--", "sh", "--connect", "user@host"},
		},
		{
			name:        "flags before double dash are applied",
			args:        []string{"__complete", "--connect", "user@host", "exec", "svc", "--", "sh", "-c", "env"},
			wantConnect: "user@host",
			wantChanged: []string{"connect"},
		},
		{
			name:        "partial flag name being completed is excluded",
			args:        []string{"__complete", "--connect", "user@host", "inspect", "--context"},
			wantConnect: "user@host",
			wantChanged: []string{"connect"},
		},
		{
			name: "partial flag value being completed is excluded",
			args: []string{"__complete", "--uncloud-config", "/tmp/"},
		},
		{
			name: "partial connect value being completed is excluded",
			args: []string{"__complete", "--connect", "tcp://127.0.0.1:5"},
		},
		{
			name:           "completed flag value with partial command word",
			args:           []string{"__complete", "--uncloud-config", "/tmp/uncloud.yaml", "insp"},
			wantConfigPath: "/tmp/uncloud.yaml",
			wantChanged:    []string{"uncloud-config"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mirror the global persistent flags defined on the root command.
			var opts globalOptions
			flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
			flags.StringVar(&opts.connect, "connect", "", "")
			flags.StringVarP(&opts.context, "context", "c", "", "")
			flags.StringVar(&opts.configPath, "uncloud-config", defaultConfigPath, "")

			applyGlobalFlagsFromCompletionArgs(flags, tt.args)

			assert.Equal(t, tt.wantConnect, opts.connect)
			assert.Equal(t, tt.wantContext, opts.context)
			wantConfigPath := tt.wantConfigPath
			if wantConfigPath == "" {
				wantConfigPath = defaultConfigPath
			}
			assert.Equal(t, wantConfigPath, opts.configPath)

			for _, name := range []string{"connect", "context", "uncloud-config"} {
				assert.Equal(t, slices.Contains(tt.wantChanged, name), flags.Changed(name),
					"changed status of flag '%s'", name)
			}
		})
	}
}
