package main

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/suite"
)

type mainTestSuite struct {
	suite.Suite
}

func (s *mainTestSuite) SetupSuite() {
	topContext = context.TODO()
}

func (s *mainTestSuite) TearDownSuite() {
}

func (s *mainTestSuite) SetupTest() {
}

func (s *mainTestSuite) TearDownTest() {
}

func (s *mainTestSuite) Test_main() {
	os.Args = []string{"pumba", "-v"}
	main()
}

func (s *mainTestSuite) Test_handleSignals() {
	handleSignals()
}

func TestMainTestSuite(t *testing.T) {
	suite.Run(t, new(mainTestSuite))
}
