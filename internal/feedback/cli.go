package feedback

import (
	"cmp"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const usage = `usage:
  columbo-feedback collect --repo owner/name [--repo owner/name ...] [--store path] [--api-url url]
  columbo-feedback status [--store path]`

type Environment struct {
	Getenv         func(string) string
	GitHubCLIToken func(hostname string) (string, error)
}

func DefaultEnvironment() Environment {
	return Environment{Getenv: os.Getenv, GitHubCLIToken: func(hostname string) (string, error) {
		args := []string{"auth", "token"}
		if hostname != "" {
			args = append(args, "--hostname", hostname)
		}
		output, err := exec.Command("gh", args...).Output()
		return strings.TrimSpace(string(output)), err
	}}
}

func Main(args []string, stdout, stderr io.Writer, env Environment) int {
	c := cli{stdout: stdout, stderr: stderr, env: env}
	commands := map[string]func([]string) int{"collect": c.collect}
	if len(args) == 0 || commands[args[0]] == nil {
		fmt.Fprintln(stderr, usage)
		return 2
	}
	return commands[args[0]](args[1:])
}

type cli struct {
	stdout, stderr io.Writer
	env            Environment
}

func (c cli) fail(code int, err error) int {
	fmt.Fprintln(c.stderr, "columbo-feedback:", err)
	return code
}

var repositoryPattern = regexp.MustCompile(`^[\w.-]+/[\w.-]+$`)

type repositoryList []string

func (r *repositoryList) String() string { return strings.Join(*r, ",") }
func (r *repositoryList) Set(value string) error {
	if !repositoryPattern.MatchString(value) {
		return fmt.Errorf("want owner/name, got %q", value)
	}
	*r = append(*r, value)
	return nil
}

func (c cli) collect(args []string) int {
	options, err := c.collectOptions(args)
	if err != nil {
		return 2
	}
	if err = Collect(context.Background(), options, c.stdout); err != nil {
		return c.fail(1, err)
	}
	return 0
}

func (c cli) collectOptions(args []string) (CollectOptions, error) {
	flags := flag.NewFlagSet("collect", flag.ContinueOnError)
	flags.SetOutput(c.stderr)
	options := CollectOptions{Now: time.Now}
	flags.Var((*repositoryList)(&options.Repositories), "repo", "repository as owner/name; repeat for more")
	flags.StringVar(&options.StorePath, "store", defaultStore(), "feedback store path")
	flags.StringVar(&options.APIURL, "api-url", cmp.Or(c.env.Getenv("GITHUB_API_URL"), "https://api.github.com"), "GitHub REST API URL")
	if err := flags.Parse(args); err != nil {
		return options, err
	}
	if len(options.Repositories) == 0 || flags.NArg() > 0 {
		fmt.Fprintln(c.stderr, usage)
		return options, errors.New(usage)
	}
	var err error
	if options.Token, err = resolveToken(options.APIURL, c.env); err != nil {
		fmt.Fprintln(c.stderr, "columbo-feedback:", err)
	}
	return options, err
}

func resolveToken(apiURL string, env Environment) (string, error) {
	if token := env.Getenv("GH_TOKEN"); token != "" {
		return token, nil
	}
	token, err := env.GitHubCLIToken(ghHostname(apiURL))
	if err != nil || token == "" {
		return "", errors.New("no GitHub token: set GH_TOKEN or run gh auth login")
	}
	return token, nil
}

func ghHostname(apiURL string) string {
	parsed, err := url.Parse(apiURL)
	if err != nil || parsed.Hostname() == "api.github.com" {
		return ""
	}
	return strings.TrimPrefix(parsed.Hostname(), "api.")
}

func defaultStore() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".columbo", "feedback.sqlite")
	}
	return filepath.Join(home, ".columbo", "feedback.sqlite")
}
