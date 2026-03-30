package model

import (
	"fmt"
	"math/rand"
	"time"
)

var sourceTypes = []string{
	"auth",
	"dns",
	"network",
	"process",
	"file",
	"web",
}

var eventCodes = map[string][]string{
	"auth": {
		"LOGIN_SUCCESS",
		"LOGIN_FAILED",
		"LOGOUT",
	},
	"dns": {
		"DNS_QUERY",
		"DNS_NXDOMAIN",
	},
	"network": {
		"CONNECTION_ALLOWED",
		"CONNECTION_DENIED",
	},
	"process": {
		"PROCESS_START",
		"PROCESS_EXIT",
	},
	"file": {
		"FILE_READ",
		"FILE_WRITE",
		"FILE_DELETE",
	},
	"web": {
		"HTTP_200",
		"HTTP_403",
		"HTTP_500",
	},
}

func randomIP() string {
	return fmt.Sprintf("10.0.%d.%d", rand.Intn(10), rand.Intn(254)+1)
}

func randomHost() string {
	return fmt.Sprintf("host-%d", rand.Intn(20)+1)
}

func randomUser() string {
	users := []string{"admin", "root", "svc_app", "analyst", "user1", "user2", "guest"}
	return users[rand.Intn(len(users))]
}

func GenerateEvent(id string) Event {
	sourceType := sourceTypes[rand.Intn(len(sourceTypes))]
	codes := eventCodes[sourceType]
	code := codes[rand.Intn(len(codes))]
	severity := rand.Intn(5) + 1
	now := time.Now().UTC()

	return Event{
		ID:          id,
		Timestamp:   now,
		SourceType:  sourceType,
		Host:        randomHost(),
		UserName:    randomUser(),
		SrcIP:       randomIP(),
		DstIP:       randomIP(),
		EventCode:   code,
		Severity:    severity,
		Message:     fmt.Sprintf("%s event from %s", code, sourceType),
		Raw:         fmt.Sprintf("raw log: type=%s code=%s host=%s", sourceType, code, randomHost()),
		GeneratedAt: now,
	}
}