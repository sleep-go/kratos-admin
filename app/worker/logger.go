package worker

import (
	"os"

	"github.com/go-kratos/kratos/v2/log"
)

func newLogger() log.Logger {
	return log.With(
		log.NewStdLogger(os.Stdout),
		"ts", log.DefaultTimestamp,
		"caller", log.DefaultCaller,
		"component", "worker",
	)
}
