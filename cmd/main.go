package main

import (
	"time"

	"github.com/nibiruchain/compounder"
	"github.com/nibiruchain/compounder/config"
)

func main() {
	config.InitConfig()

	comp := compounder.NewCompounder()
	comp.ClaimRewards()

	time.Sleep(10 * time.Second)

	comp.Compound()
}
