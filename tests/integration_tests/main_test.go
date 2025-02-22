package integrationtests

import (
	"log"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	log.SetOutput(os.Stdout)

	m.Run()
}
