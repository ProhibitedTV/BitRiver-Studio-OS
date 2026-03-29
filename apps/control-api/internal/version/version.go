package version

var (
	buildVersion = "dev"
	buildCommit  = "local"
)

func String() string {
	return buildVersion + "+" + buildCommit
}
