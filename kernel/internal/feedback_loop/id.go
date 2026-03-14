package feedbackloop

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

func NewID(prefix string) string {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		panic(err)
	}

	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(raw[:]))
}

func NewIncidentID() string {
	return NewID("inc")
}

func NewRequestEventID() string {
	return NewID("req")
}

func NewTestRunID() string {
	return NewID("testrun")
}

func NewAgentReviewID() string {
	return NewID("review")
}
