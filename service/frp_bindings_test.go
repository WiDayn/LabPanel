package service

import (
	"LabPanel/config"
	"LabPanel/models"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const legacyFRPFixture = `serverAddr = "127.0.0.1"
serverPort = 7000
includes = ["./extra/*.toml"]

[transport]
tcpMux = false

[[proxies]]
name = "old"
type = "tcp"
localIP = "10.42.0.10"
localPort = 22
remotePort = 20022
[proxies.transport]
useEncryption = true
`

const frpTestInstances = `[{"name":"original","type":"container","status":"Running","config":{"volatile.uuid":"original-id","volatile.eth0.hwaddr":"00:16:3e:00:00:10"},"expanded_devices":{"eth0":{"type":"nic","name":"eth0","network":"testbridge","ipv4.address":"10.42.0.10"}},"state":{"network":{"eth0":{"hwaddr":"00:16:3e:00:00:10","addresses":[{"family":"inet","scope":"global","address":"10.42.0.10"}]}}}}]`

func writeFRPTestFile(t *testing.T, path, contents string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), mode); err != nil {
		t.Fatal(err)
	}
}

func TestFRPBindingPreservesLegacyFieldsAndPausesOnAddressReassignment(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "frpc.toml")
	instancePath := filepath.Join(dir, "instances.json")
	frpcPath := filepath.Join(dir, "frpc")
	logPath := filepath.Join(dir, "calls")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FRP_TEST_INSTANCES", instancePath)
	t.Setenv("FRP_TEST_CALLS", logPath)
	writeFRPTestFile(t, configPath, legacyFRPFixture, 0600)
	writeFRPTestFile(t, instancePath, frpTestInstances, 0600)
	writeFRPTestFile(t, filepath.Join(dir, "proxy_comments.json"), `{"old":"legacy remark"}`, 0600)
	writeFRPTestFile(t, frpcPath, "#!/bin/sh\nprintf '%s\\n' \"$1\" >> \"$FRP_TEST_CALLS\"\n", 0700)
	writeFRPTestFile(t, filepath.Join(dir, "lxc"), `#!/bin/sh
case "$2 $3" in
  'list --project=default') cat "$FRP_TEST_INSTANCES" ;;
  'query /1.0/networks/testbridge?project=default') printf '%s\n' '{"type":"bridge","managed":true}' ;;
  'network list-leases') printf '%s\n' '[{"address":"10.42.0.10","hwaddr":"00:16:3e:00:00:10"}]' ;;
  *) exit 1 ;;
esac
`, 0700)
	service := &FrpConfigService{cfg: &config.Config{TomlPath: configPath, FrpcPath: frpcPath}}
	legacy, err := service.GetConfig()
	if err != nil || len(legacy.Proxies) != 1 || legacy.Proxies[0].Candidate != "original" || legacy.Proxies[0].Binding != nil {
		t.Fatalf("legacy import: %+v, %v", legacy, err)
	}
	proxy := legacy.Proxies[0]
	proxy.Binding = &models.ProxyBinding{InstanceID: "original-id", Container: "original", Device: "eth0", Address: "10.42.0.10"}
	stale := *proxy.Binding
	stale.Address = "10.42.0.99"
	if _, err := service.ConfirmBinding("old", stale); err == nil {
		t.Fatal("confirmed a rule whose address changed")
	}
	if _, err := service.ConfirmBinding("old", *proxy.Binding); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ConfirmBinding("old", *proxy.Binding); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(configPath)
	for _, field := range []string{"tcpMux = false", "useEncryption = true", "includes =", "remotePort = 20022"} {
		if !strings.Contains(string(data), field) {
			t.Fatalf("configuration lost %q", field)
		}
	}
	calls, _ := os.ReadFile(logPath)
	if strings.Contains(string(calls), "reload") {
		t.Fatal("metadata-only binding reloaded FRP")
	}
	writeFRPTestFile(t, instancePath, strings.ReplaceAll(frpTestInstances, "original-id", "replacement-id"), 0600)
	if err := service.reconcileBindings(); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(configPath)
	if strings.Contains(string(data), "[[proxies]]") {
		t.Fatal("proxy remains enabled after address changed identity")
	}
	paused, err := service.GetConfig()
	if err != nil || len(paused.Proxies) != 1 || !paused.Proxies[0].Paused || paused.Proxies[0].Comment != "legacy remark" {
		t.Fatalf("paused proxy: %+v, %v", paused, err)
	}
	writeFRPTestFile(t, instancePath, frpTestInstances, 0600)
	if err := service.reconcileBindings(); err != nil {
		t.Fatal(err)
	}
	resumed, err := service.GetConfig()
	if err != nil || resumed.Proxies[0].Paused || resumed.Proxies[0].Binding.InstanceID != "original-id" {
		t.Fatalf("resumed proxy: %+v, %v", resumed, err)
	}
}

func TestFRPReloadFailureRestoresOriginalBytes(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "frpc.toml")
	frpcPath := filepath.Join(dir, "frpc")
	writeFRPTestFile(t, configPath, legacyFRPFixture, 0600)
	writeFRPTestFile(t, frpcPath, `#!/bin/sh
if [ "$1" = reload ] && grep -q 'name = .new.' "$3"; then exit 1; fi
`, 0700)
	service := &FrpConfigService{cfg: &config.Config{TomlPath: configPath, FrpcPath: frpcPath}}
	err := service.AddProxy(models.Proxy{Name: "new", Type: "tcp", LocalIP: "127.0.0.1", LocalPort: 22, RemotePort: 20023})
	if err == nil {
		t.Fatal("reload failure was not reported")
	}
	data, err := os.ReadFile(configPath)
	if err != nil || string(data) != legacyFRPFixture {
		t.Fatalf("original FRP configuration not restored: %v", err)
	}
}

func TestFRPUpgradeRejectsEnabledLegacyForward(t *testing.T) {
	path := filepath.Join(t.TempDir(), "port_forwards.json")
	cfg := &config.Config{PortForwardsPath: path}
	if err := CheckFRPUpgrade(cfg); err != nil {
		t.Fatal(err)
	}
	writeFRPTestFile(t, path, `{"forwards":[{"name":"legacy","listenAddress":"127.0.0.1","listenPort":8088,"enabled":true}]}`, 0600)
	if err := CheckFRPUpgrade(cfg); err == nil || !strings.Contains(err.Error(), "127.0.0.1:8088") {
		t.Fatalf("enabled legacy rules were not reported: %v", err)
	}
	writeFRPTestFile(t, path, `{"forwards":[{"name":"legacy","enabled":false}]}`, 0600)
	if err := CheckFRPUpgrade(cfg); err != nil {
		t.Fatal(err)
	}
}
