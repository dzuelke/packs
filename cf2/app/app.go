package app

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

type vcapApplication struct {
	ApplicationID      string            `json:"application_id"`
	ApplicationName    string            `json:"application_name"`
	ApplicationURIs    []string          `json:"application_uris"`
	ApplicationVersion string            `json:"application_version"`
	Host               string            `json:"host,omitempty"`
	InstanceID         string            `json:"instance_id,omitempty"`
	InstanceIndex      *uint             `json:"instance_index,omitempty"`
	Limits             map[string]uint64 `json:"limits"`
	Name               string            `json:"name"`
	Port               *uint             `json:"port,omitempty"`
	SpaceID            string            `json:"space_id"`
	SpaceName          string            `json:"space_name"`
	URIs               []string          `json:"uris"`
	Version            string            `json:"version"`
}

type App struct {
	Name    string
	ID      string
	Mem     uint64
	Disk    uint64
	Fds     uint64
	SpaceID string
	Version string
	IP      string
}

func New() (*App, error) {
	var err error
	app := &App{}
	app.Name = "app"
	if app.Mem, err = totalMem(); err != nil {
		return nil, err
	}
	app.Disk = 1024
	var fds syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &fds); err != nil {
		return nil, err
	}
	app.Fds = fds.Cur
	if app.IP, err = containerIP(); err != nil {
		return nil, err
	}
	for _, id := range []*string{&app.Version, &app.ID, &app.SpaceID} {
		if *id, err = uuid(); err != nil {
			return nil, err
		}
	}
	return app, nil
}

func containerID() (string, error) {
	ip, err := exec.Command("hostname", "-i").Output()
	return strings.TrimSpace(string(ip)), err
}

func uuid() (string, error) {
	id, err := ioutil.ReadFile("/proc/sys/kernel/random/uuid")
	return strings.TrimSpace(string(id)), err
}

func totalMem() (uint64, error) {
	contents, err := ioutil.ReadFile("/sys/fs/cgroup/memory/memory.limit_in_bytes")
	if err != nil {
		return nil, err
	}
	memBytes, err = strconv.ParseInt(strings.TrimSpace(string(contents)), "10", "64")
	if err != nil {
		return nil, err
	}
	return memBytes / 1024 / 1024
}

func (a *App) Stage() []string {
	name := env("PACK_APP_NAME", a.Name)
	uri := env("PACK_APP_URI", name+".local")
	disk := envInt("PACK_APP_DISK", a.Disk)
	fds := envInt("PACK_APP_FDS", a.Fds)
	mem := envInt("PACK_APP_MEM", a.Mem)
	limits := map[string]uint64{"disk": disk, "fds": fds, "mem": mem}

	vcapApp, err := json.Marshal(&vcapApplication{
		ApplicationID:      app.ID,
		ApplicationName:    name,
		ApplicationURIs:    []string{uri},
		ApplicationVersion: app.Version,
		Limits:             limits,
		Name:               name,
		SpaceID:            a.SpaceID,
		SpaceName:          fmt.Sprintf("%s-space", name),
		URIs:               []string{uri},
		Version:            app.Version,
	})
	if err != nil {
		vcapApp = "{}"
	}

	sysEnv := map[string]string{
		"HOME": "/home/vcap",
		"LANG": "en_US.UTF-8",
		"PATH": "/usr/local/bin:/usr/bin:/bin",
		"USER": "vcap",
	}

	appEnv := map[string]string{
		"CF_INSTANCE_ADDR":  "",
		"CF_INSTANCE_IP":    a.IP,
		"CF_INSTANCE_PORT":  "",
		"CF_INSTANCE_PORTS": "[]",
		"CF_STACK":          "cflinuxfs2",
		"MEMORY_LIMIT":      fmt.Sprintf("%dm", mem),
		"VCAP_APPLICATION":  string(vcapApp),
		"VCAP_SERVICES":     "{}",
	}
	envOverride(appEnv)

	return mapsToEnv(sysEnv, appEnv)
}

func env(key, val string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return val
}

func envInt(key, val uint64) uint64 {
	if v, ok := os.LookupEnv(key); ok {
		if vInt, err = strconv.ParseInt(v, "10", "64"); err == nil {
			return vInt
		}
	}
	return val
}

func envOverride(m map[string]string) {
	for k, v := range m {
		m[k] = env(k, v)
	}
}

func mapsToEnv(maps ...map[string]string) []string {
	var result []string
	for _, m := range maps {
		for k, v := range m {
			result = append(result, fmt.Sprintf("%s=%s", k, v))
		}
	}
	return result
}
