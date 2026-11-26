package main

import (
	"net/netip"
	"regexp"
	"strconv"
	"time"
)

// LogRecord содержит поля одной записи NGINX access log.
type LogRecord struct {
	IP                         netip.Addr
	Time                       time.Time
	Method, Resource, Protocol string
	Status                     int
	Size                       int64
}

var accessLine = regexp.MustCompile(`^(\S+) - \S+ \[([^\]]+)\] "([^ "\t]+) ([^ "\t]+) (HTTP/[0-9]+(?:\.[0-9]+)?)" ([1-5][0-9]{2}) ([0-9]+) "(?:[^"\\]|\\.)*" "(?:[^"\\]|\\.)*"$`)

func parseRecord(line string) (LogRecord, bool) {
	fields := accessLine.FindStringSubmatch(line)
	if fields == nil {
		return LogRecord{}, false
	}
	ip, err := netip.ParseAddr(fields[1])
	if err != nil {
		return LogRecord{}, false
	}
	stamp, err := time.Parse("02/Jan/2006:15:04:05 -0700", fields[2])
	if err != nil {
		return LogRecord{}, false
	}
	size, err := strconv.ParseInt(fields[7], 10, 64)
	if err != nil {
		return LogRecord{}, false
	}
	status, _ := strconv.Atoi(fields[6]) // Выражение допускает ровно три цифры.
	return LogRecord{
		IP: ip, Time: stamp, Method: fields[3], Resource: fields[4],
		Protocol: fields[5], Status: status, Size: size,
	}, true
}

func (o options) includes(stamp time.Time) bool {
	year, month, day := stamp.Date()
	date := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	return (o.from == nil || !date.Before(*o.from)) && (o.to == nil || !date.After(*o.to))
}
