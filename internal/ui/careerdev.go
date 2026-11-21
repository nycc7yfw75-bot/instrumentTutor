package ui

import (
	"math/rand"
	"os"
	"time"
)

// forceTough is the HEADLINER_FORCE_TOUGH=1 dev flag (CAREER.md §10):
// every gig rolls a tough crowd, to test booing without waiting for luck.
func forceTough() bool { return os.Getenv("HEADLINER_FORCE_TOUGH") == "1" }

// gigRand is the randomness for a gig's crowd roll.
func gigRand() *rand.Rand { return rand.New(rand.NewSource(time.Now().UnixNano())) }
