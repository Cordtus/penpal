package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/cordtus/penpal/internal/scan"
	"github.com/cordtus/penpal/internal/settings"
)

func main() {
	var (
		file string
		init bool
	)
	flag.BoolVar(&init, "init", false, "initialize a new config file")
	flag.StringVar(&file, "config", "./config.json", "path to the config file")
	flag.StringVar(&file, "c", "./config.json", "path to the config file [shorthand]")
	flag.Parse()
	args := os.Args
	if len(args) > 1 && !strings.HasPrefix(args[1], "-") {
		fmt.Println("invalid argument:", os.Args[1])
		flag.Usage()
		return
	}
	if init {
		if err := settings.New(file); err != nil {
			fmt.Println(err)
		}
		return
	}
	cfg, err := settings.Load(file)
	if err != nil {
		log.Fatal("Failed to load configuration:", err)
	}

	network := cfg.Network[0]

	if network.StallTime == 1 {
		fmt.Println("warning! stall time for", network.ChainId, "is set to 1 minute, this may cause more frequent false alerts")
	} else if network.StallTime == 0 {
		fmt.Println("warning! stall check for", network.ChainId, "is disabled")
	}
	if !network.RpcAlert {
		fmt.Println("warning! rpc alerts for", network.ChainId, "are disabled")
	}

	scan.Monitor(cfg)
}
