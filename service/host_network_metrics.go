package service

import (
	"LabPanel/models"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type hostNetworkCounters struct {
	IfIndex        string
	CarrierChanges string
	LinkUp         bool
	RxBytes        int64
	TxBytes        int64
}

func readHostNetworkPorts(netDev, sysClassNet string) map[string]hostNetworkCounters {
	data, err := os.ReadFile(netDev)
	if err != nil {
		return nil
	}
	ports := make(map[string]hostNetworkCounters)
	for _, line := range strings.Split(string(data), "\n") {
		iface, counters, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		iface = strings.TrimSpace(iface)
		fields := strings.Fields(counters)
		if iface == "" || len(fields) < 16 {
			continue
		}
		dir := filepath.Join(sysClassNet, iface)
		if !isPhysicalEthernet(dir) {
			continue
		}
		rx, rxErr := strconv.ParseInt(fields[0], 10, 64)
		tx, txErr := strconv.ParseInt(fields[8], 10, 64)
		if rxErr != nil || txErr != nil || rx < 0 || tx < 0 {
			continue
		}
		ports[iface] = hostNetworkCounters{
			IfIndex:        readNetworkAttribute(dir, "ifindex"),
			CarrierChanges: readNetworkAttribute(dir, "carrier_changes"),
			LinkUp:         readNetworkAttribute(dir, "carrier") == "1",
			RxBytes:        rx,
			TxBytes:        tx,
		}
	}
	return ports
}

func readNetworkAttribute(dir, name string) string {
	data, _ := os.ReadFile(filepath.Join(dir, name))
	return strings.TrimSpace(string(data))
}

func isPhysicalEthernet(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, "device")); err != nil {
		return false
	}
	if readNetworkAttribute(dir, "type") != "1" {
		return false
	}
	for _, attr := range []string{"wireless", "phy80211", "device/physfn"} {
		if _, err := os.Lstat(filepath.Join(dir, attr)); err == nil {
			return false
		}
	}
	return true
}

func addHostNetworkMetrics(point *models.HostMetricsPoint, current, previous map[string]hostNetworkCounters, elapsed float64) {
	if current == nil {
		return
	}
	point.NetworkPorts = make([]models.HostNetworkPort, 0, len(current))
	names := make([]string, 0, len(current))
	for name := range current {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		raw := current[name]
		before, hasBefore := previous[name]
		point.NetworkRxBytes += raw.RxBytes
		point.NetworkTxBytes += raw.TxBytes
		port := models.HostNetworkPort{Name: name, LinkUp: raw.LinkUp}
		if hasBefore && elapsed > 0 && raw.IfIndex != "" && raw.IfIndex == before.IfIndex {
			// Keep the total across all physical ports, even if a cable was unplugged
			// during this interval. The port curve only uses stable, connected intervals.
			linkStable := raw.LinkUp && before.LinkUp && raw.CarrierChanges == before.CarrierChanges
			if raw.RxBytes >= before.RxBytes {
				rate := positiveRate(raw.RxBytes, before.RxBytes, elapsed)
				point.NetworkRxBps += rate
				if linkStable {
					port.RxBps = &rate
				}
			}
			if raw.TxBytes >= before.TxBytes {
				rate := positiveRate(raw.TxBytes, before.TxBytes, elapsed)
				point.NetworkTxBps += rate
				if linkStable {
					port.TxBps = &rate
				}
			}
		}
		point.NetworkPorts = append(point.NetworkPorts, port)
	}
}

func averageHostNetworkPorts(points []models.HostMetricsPoint) []models.HostNetworkPort {
	last := points[len(points)-1].NetworkPorts
	if last == nil {
		return nil
	}
	type rates struct {
		rx, tx           float64
		rxCount, txCount int
	}
	byName := make(map[string]rates, len(last))
	for _, point := range points {
		for _, port := range point.NetworkPorts {
			v := byName[port.Name]
			if port.RxBps != nil {
				v.rx += *port.RxBps
				v.rxCount++
			}
			if port.TxBps != nil {
				v.tx += *port.TxBps
				v.txCount++
			}
			byName[port.Name] = v
		}
	}
	ports := make([]models.HostNetworkPort, 0, len(last))
	for _, port := range last {
		v := byName[port.Name]
		sample := models.HostNetworkPort{Name: port.Name, LinkUp: port.LinkUp}
		// A missing or disconnected sample makes a gap even in a downsampled bucket.
		if v.rxCount == len(points) {
			rate := v.rx / float64(v.rxCount)
			sample.RxBps = &rate
		}
		if v.txCount == len(points) {
			rate := v.tx / float64(v.txCount)
			sample.TxBps = &rate
		}
		ports = append(ports, sample)
	}
	return ports
}
