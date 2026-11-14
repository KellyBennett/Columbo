package columbo

import "regexp"

var machineCommentPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^//go:(build|generate|embed|linkname|wasmimport|wasmexport|debug|cgo_export_static|cgo_export_dynamic|cgo_import_dynamic|cgo_import_static|cgo_dynamic_linker|cgo_ldflag)[\t ]+\S.*$`),
	regexp.MustCompile(`^//go:(nointerface|noescape|norace|nosplit|noinline|nocheckptr|systemstack|nowritebarrier|nowritebarrierrec|yeswritebarrierrec|cgo_unsafe_args|uintptrkeepalive|uintptrescapes|registerparams)[\t ]*$`),
	regexp.MustCompile(`^//[\t ]*\+build[\t ]+\S.*$`),
	regexp.MustCompile(`^(//line [^\r\n]+:[1-9][0-9]*(:[1-9][0-9]*)?|/\*line [^\r\n]+:[1-9][0-9]*(:[1-9][0-9]*)?\*/)$`),
	regexp.MustCompile(`^//export [\pL_][\pL\pN_]*$`),
	regexp.MustCompile(`^//lint:(ignore|file-ignore)[\t ]+\S+[\t ]+\S.*$`),
	regexp.MustCompile(`^//[\t ]*columbo:ignore[\t ]+[^\t ]+[\t ]+--[\t ]+\S.*$`),
}

var nolintDirective = regexp.MustCompile(`^/{2}nolint(?::[a-zA-Z0-9_-]+(?:,[a-zA-Z0-9_-]+)*)?(?:[\t ]+(//[^\r\n]*))?$`)

func proseOffset(text string) (int, bool) {
	if match := nolintDirective.FindStringSubmatchIndex(text); match != nil {
		return match[2], match[2] >= 0
	}
	return 0, !machineComment(text)
}

func machineComment(text string) bool {
	for _, pattern := range machineCommentPatterns {
		if pattern.MatchString(text) {
			return true
		}
	}
	return false
}
