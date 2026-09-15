module github.com/LSFLK/argus/pkg/audit

go 1.24.6

// v1.0.0 was tagged before the team agreed the client API is stable.
// Do not move or delete pkg/audit/v1.0.0: proxy.golang.org and
// sum.golang.org already cached it.
// v1.0.1 exists only so the go command can discover these retract
// directives (it reads them from the highest release, even if retracted).
// Consumers should use a 0.x tag (`go get …@latest` skips retracted 1.x).
retract (
	v1.0.0
	v1.0.1
)
