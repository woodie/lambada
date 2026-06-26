package main

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestLambadaMTA(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Lambada MTA Suite")
}
