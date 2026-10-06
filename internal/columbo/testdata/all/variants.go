package fixture

type SourceKind string

const (
	RSS     SourceKind = "rss"
	Reuters SourceKind = "reuters"
)

func Fetch(kind SourceKind) {
	switch kind {
	case RSS:
		println("fetch rss")
	case Reuters:
		println("fetch reuters")
	}
}
func Check(kind SourceKind) {
	if kind == RSS {
		println("rss health")
	} else if kind == Reuters {
		println("credentials")
	}
}
