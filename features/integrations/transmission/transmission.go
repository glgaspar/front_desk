package transmission

import (
	"context"
	"fmt"
	"net/url"

	"github.com/glgaspar/front_desk/connection"
	"github.com/hekmon/transmissionrpc/v3"
)

type Config struct {
	Url      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password"`
	Port     int    `json:"port"`
}

func (c *Config) SetTransmission() error {
	conn, err := connection.Db()
	if err != nil {
		return err
	}
	defer conn.Close()

	tran, err := conn.Begin()
	if err != nil {
		return err
	}

	query := `
	delete from adm.transmission;
	`
	_, err = tran.Exec(query)
	if err != nil {
		tran.Rollback()
		return err
	}

	query = `
	insert into adm.transmission (url, username, password, port)
	values ($1, $2, $3, $4);
	`
	_, err = tran.Exec(query, c.Url, c.Username, c.Password, c.Port)
	if err != nil {
		tran.Rollback()
		return err
	}

	return tran.Commit()
}

type Transmission struct {
	Client *transmissionrpc.Client `json:"client"`
}

func (t *Transmission) Connect() error {
	config := Config{}
	conn, err := connection.Db()
	if err != nil {
		return err
	}
	defer conn.Close()

	query := `
	select url, username, password, port
	from adm.transmission
	limit 1;
	`

	err = conn.QueryRow(query).Scan(&config.Url, &config.Username, &config.Password, &config.Port)
	if err != nil {
		return err
	}

	endpoint, err := url.Parse(fmt.Sprintf("http://%s:%s@%s:%d/transmission/rpc", config.Username, config.Password, config.Url, config.Port))
	if err != nil {
		panic(err)
	}

	t.Client, err = transmissionrpc.New(endpoint, nil)
	if err != nil {
		panic(err)
	}

	return nil
}

func (t *Transmission) ValidadeVersion() error {
	ok, serverVersion, serverMinimumVersion, err := t.Client.RPCVersion(context.TODO())
	if err != nil {
		panic(err)
	}
	if !ok {
		return fmt.Errorf("Remote transmission RPC version (v%d) is incompatible with the transmission library (v%d): remote needs at least v%d",
			serverVersion, transmissionrpc.RPCVersion, serverMinimumVersion)
	}
	fmt.Printf("Remote transmission RPC version (v%d) is compatible with our transmissionrpc library (v%d)\n",
		serverVersion, transmissionrpc.RPCVersion)
	return nil
}

func (t *Transmission) GetAllTorrents() (*[]transmissionrpc.Torrent, error) {
	err := t.Connect()
	if err != nil {
		return nil, err
	}

	err = t.ValidadeVersion()
	if err != nil {
		return nil, err
	}

	torrents, err := t.Client.TorrentGetAll(context.TODO())
	if err != nil {
		return nil, err
	}

	return &torrents, nil
}

func (t *Transmission) ToggleTorrent(id int64, action string) error {
	err := t.Connect()
	if err != nil {
		return err
	}

	err = t.ValidadeVersion()
	if err != nil {
		return err
	}

	ids := []int64{id}
	
	switch action {
		case "start":
			err = t.Client.TorrentStartIDs(context.TODO(), ids)
		case "stop":
			err = t.Client.TorrentStopIDs(context.TODO(), ids)
		default:
			return fmt.Errorf("unknown action: %s", action)
	}
	if err != nil {
		return err
	}

	return nil
}