package main

import (
	"fmt"
	"strconv"
	"strings"
)

func dbNodeID(connID string, index int) string {
	return fmt.Sprintf("%s/db/%d", connID, index)
}

func parseDBNode(id string) (connID string, db int, ok bool) {
	i := strings.LastIndex(id, "/db/")
	if i <= 0 {
		return "", 0, false
	}
	db, err := strconv.Atoi(id[i+4:])
	if err != nil {
		return "", 0, false
	}
	return id[:i], db, true
}

func navConnID(id string) string {
	if connID, _, ok := parseDBNode(id); ok {
		return connID
	}
	return id
}
