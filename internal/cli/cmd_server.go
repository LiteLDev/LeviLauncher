package cli

import (
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/liteldev/LeviLauncher/internal/mcservice"
)

// Seams replaced by tests.
var (
	serverListFn = mcservice.ListServers
	serverPingFn = mcservice.MotdBE
)

// serverPingWorkers bounds the concurrent pings.
const serverPingWorkers = 8

func init() {
	Register(&Command{
		Path:    "server list",
		Summary: "list the saved servers and check whether they are online",
		Args:    "[<filter>]",
		Description: `Shows the external servers saved in the game's server list, in the game's
order, and pings each one (players, version, latency). STATUS is online,
offline (no answer within 3 seconds) or unreachable (the address does not
resolve). <filter> matches part of the name or address, ignoring case.`,
		Examples: []string{"server list", "server list -i Survival -p Steve --json", "server ls --no-ping"},
		Flags:    []*Flag{BoolFlag("ping", "", "contact each server; --no-ping only reads the list").WithDefault("true")},
		MaxArgs:  1,
		Run:      runServerList,
	})
}

type serverItem struct {
	Name       string    `json:"name"`
	Host       string    `json:"host"`
	Port       int       `json:"port"`
	Added      time.Time `json:"added"`
	Status     string    `json:"status,omitempty"`
	Motd       string    `json:"motd,omitempty"`
	Version    string    `json:"version,omitempty"`
	Protocol   int       `json:"protocol,omitempty"`
	Players    *int      `json:"players,omitempty"`
	MaxPlayers *int      `json:"maxPlayers,omitempty"`
	LatencyMs  *int64    `json:"latencyMs,omitempty"`
}

func (s serverItem) address() string {
	return net.JoinHostPort(s.Host, strconv.Itoa(s.Port))
}

func runServerList(c *Ctx) error {
	inst, err := c.ResolveInstance("")
	if err != nil {
		return err
	}
	player, err := c.ResolvePlayer(inst)
	if err != nil {
		return err
	}
	list, err := serverListFn(inst.Name, player.ID)
	if err != nil {
		return Errorf(ExitFailure, "ERR_READ_SERVERS", "cannot read the server list of instance %s: %v", inst.Name, err)
	}
	filter := strings.ToLower(strings.TrimSpace(c.Arg(0)))
	items := []serverItem{}
	for _, s := range list {
		it := serverItem{Name: s.Name, Host: strings.TrimSpace(s.IP)}
		it.Port, _ = strconv.Atoi(strings.TrimSpace(s.Port))
		if s.Timestamp > 0 {
			it.Added = time.Unix(s.Timestamp, 0)
		}
		if filter != "" && !strings.Contains(strings.ToLower(it.Name+"\x00"+it.address()), filter) {
			continue
		}
		items = append(items, it)
	}
	if c.Bool("ping") && len(items) > 0 {
		if err := serverPingAll(c, items); err != nil {
			return err
		}
	}
	if len(items) == 0 {
		c.Notice("No saved servers.")
	}
	ids := make([]string, len(items))
	for i, it := range items {
		ids[i] = it.address()
	}
	return c.Render(items, ids, func(w io.Writer) error {
		if len(items) == 0 {
			return nil
		}
		rows := make([][]string, len(items))
		for i, it := range items {
			players, latency := "-", "-"
			if it.Players != nil && it.MaxPlayers != nil {
				players = fmt.Sprintf("%d/%d", *it.Players, *it.MaxPlayers)
			}
			if it.LatencyMs != nil {
				latency = fmt.Sprintf("%d ms", *it.LatencyMs)
			}
			rows[i] = []string{it.Name, it.address(), Dash(it.Status), players, Dash(it.Version), latency, Dash(it.Motd)}
		}
		return Table(w, []string{"NAME", "ADDRESS", "STATUS", "PLAYERS", "VERSION", "LATENCY", "MOTD"}, rows)
	})
}

// serverPingAll pings every server with a bounded pool and fills in the
// results. Ctrl-C stops waiting.
func serverPingAll(c *Ctx, items []serverItem) error {
	p := c.Progress()
	p.Phase("Checking %d servers", len(items))
	jobs := make(chan int)
	var wg sync.WaitGroup
	var mu sync.Mutex
	done := 0
	for w := 0; w < min(serverPingWorkers, len(items)); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				r := serverPingOne(items[i].address())
				mu.Lock()
				serverApply(&items[i], r)
				done++
				p.Count(int64(done), int64(len(items)), "servers")
				mu.Unlock()
			}
		}()
	}
	finished := make(chan struct{})
	go func() {
		defer close(jobs)
		for i := range items {
			select {
			case jobs <- i:
			case <-c.Context().Done():
				return
			}
		}
	}()
	go func() { wg.Wait(); close(finished) }()
	select {
	case <-finished:
	case <-c.Context().Done():
		return c.Context().Err()
	}
	p.Done()
	return c.Context().Err()
}

type serverPingResult struct {
	status string
	info   *mcservice.MotdBEInfo
}

func serverPingOne(addr string) serverPingResult {
	info, err := serverPingFn(addr)
	var dnsErr *net.DNSError
	switch {
	case err != nil && errors.As(err, &dnsErr):
		return serverPingResult{status: "unreachable"}
	case info == nil || info.Status != "online":
		return serverPingResult{status: "offline"}
	}
	return serverPingResult{status: "online", info: info}
}

func serverApply(it *serverItem, r serverPingResult) {
	it.Status = r.status
	if r.info == nil {
		return
	}
	in := r.info
	it.Motd = contentPlain(in.Motd)
	it.Version = in.Version
	it.Protocol = in.Agreement
	online, maxp, delay := in.Online, in.Max, in.Delay
	it.Players, it.MaxPlayers, it.LatencyMs = &online, &maxp, &delay
}
