package feedback

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveToken(t *testing.T) {
	tests := []struct {
		name, apiURL, ghToken, cliToken string
		wantHost, wantToken             string
	}{
		{"GH_TOKEN wins", "https://api.github.com", "env-token", "cli-token", "", "env-token"},
		{"github.com asks gh without a host", "https://api.github.com", "", "cli-token", "", "cli-token"},
		{"Enterprise Server asks for its host", "https://git.example.com/api/v3", "", "cli-token", "git.example.com", "cli-token"},
		{"Enterprise Cloud drops the api prefix", "https://api.acme.ghe.com", "", "cli-token", "acme.ghe.com", "cli-token"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			askedHost := "not asked"
			env := Environment{
				Getenv: func(key string) string { return map[string]string{"GH_TOKEN": test.ghToken}[key] },
				GitHubCLIToken: func(hostname string) (string, error) {
					askedHost = hostname
					return test.cliToken, nil
				},
			}
			token, err := resolveToken(test.apiURL, env)
			require.NoError(t, err)
			require.Equal(t, test.wantToken, token)
			if test.ghToken == "" {
				require.Equal(t, test.wantHost, askedHost)
			}
		})
	}
}

func TestCollectWithoutATokenExplainsHowToGetOne(t *testing.T) {
	env := Environment{
		Getenv:         func(string) string { return "" },
		GitHubCLIToken: func(string) (string, error) { return "", errors.New("not logged in") },
	}
	var stderr bytes.Buffer
	code := Main([]string{"collect", "--repo", "o/r", "--store", t.TempDir() + "/f.sqlite"}, &bytes.Buffer{}, &stderr, env)
	require.Equal(t, 2, code)
	require.Contains(t, stderr.String(), "set GH_TOKEN or run gh auth login")
}

func TestCollectRejectsAMalformedRepository(t *testing.T) {
	var stderr bytes.Buffer
	code := Main([]string{"collect", "--repo", "just-a-name"}, &bytes.Buffer{}, &stderr, DefaultEnvironment())
	require.Equal(t, 2, code)
	require.Contains(t, stderr.String(), "want owner/name")
}
