// Copyright © 2016 Asteris, LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"log"
	"os"
	"os/signal"

	"golang.org/x/net/context"
)

// GracefulExit traps interrupt signals for a graceful exit
func GracefulExit(cancel context.CancelFunc) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		interruptCount := 0

		for range c {
			interruptCount++

			switch interruptCount {
			case 1:
				log.Println("[INFO] gracefully shutting down (interrupt again to halt)")
				cancel()
			case 2:
				log.Println("[WARN] hard stop! System may be left in an incomplete state")
				os.Exit(2)
			}
		}
	}()
}
