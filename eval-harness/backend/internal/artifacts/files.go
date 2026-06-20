// Package artifacts is the single source of run artifact filenames, shared by
// writer (orchestrator) and reader (server).
package artifacts

const (
	MetaFile       = "meta.json"
	BuildFile      = "build.txt"
	TestFile       = "test.txt"
	JudgeLogFile   = "judge.txt"
	DiffFile       = "changes.diff"
	TranscriptFile = "transcript.jsonl"
	CoderErrFile   = "claude.err"
	TelemetryFile  = "mcp-telemetry.jsonl"
	JudgeJSONFile  = "judge.json"
	CancelledFile  = "cancelled"
)
