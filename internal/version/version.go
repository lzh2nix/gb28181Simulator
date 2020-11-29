package version

import (
	"fmt"

	cli "github.com/jawher/mow.cli"
)

var (
	version = "pokémon/0.2"
)

func Version() string {
	return version
}
func Print(cli *cli.Cmd) {
	fmt.Println("version = ", Version())
}
