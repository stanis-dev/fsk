package orchestrator

// Config is the fully-resolved input for constructing a Runner. The
// orchestrator performs no path discovery; every location is supplied
// explicitly by the caller.
type Config struct {
	ScenariosDir   string
	RepoRoot       string
	DockerfilePath string
	RunsBase       string
	Image          string
	Model          string
	Effort         string
}
