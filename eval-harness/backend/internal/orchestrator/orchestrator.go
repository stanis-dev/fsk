package orchestrator

// Config is the fully-resolved input for constructing a Runner. The
// orchestrator performs no discovery of its own; every path, credential, and
// engine selection is supplied explicitly by the caller.
type Config struct {
	ScenariosDir   string
	RepoRoot       string
	DockerfilePath string
	RunsBase       string
	Image          string
	Model          string
	Effort         string
	Token          string
	DockerContext  string
}
