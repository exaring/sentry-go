package sentry

import (
	"github.com/exaring/sentry-go/internal/testutils"
)

var assertEqual = testutils.AssertEqual
var assertNotEqual = testutils.AssertNotEqual
var assertBaggageStringsEqual = testutils.AssertBaggageStringsEqual
