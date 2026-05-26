package supervisor

type Program struct {
	Name string
	Path string
	Args []string
}

func DefaultPrograms() []Program {
	return []Program{
		{Name: "postgres", Path: "/usr/local/bin/ociger-pg-launcher"},
		{Name: "nats", Path: "/usr/local/bin/nats-server", Args: []string{"--config", "/etc/nats/nats-server.conf"}},
	}
}
