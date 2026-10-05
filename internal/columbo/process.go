package columbo

import (
	"os"
)

func Main(version string) int {
	dir, e := os.Getwd()
	if e != nil {
		return processError(e, 2)
	}
	return Run(os.Args[1:], processInvocation(dir, version))
}
func processInvocation(dir, version string) Invocation {
	return Invocation{Dir: dir, Version: version, Stdout: os.Stdout, Stderr: os.Stderr, GitHubActions: os.Getenv("GITHUB_ACTIONS") == "true"}
}
func processError(e error, code int) int {
	os.Stderr.WriteString(e.Error() + "\n")
	return code
}
func ReleaseMain() int {
	if e := ValidatePublicRelease(); e != nil {
		return processError(e, 1)
	}
	return 0
}
