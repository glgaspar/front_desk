package transmission

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"time"

	"github.com/glgaspar/front_desk/connection"
	"github.com/hekmon/transmissionrpc/v3"
)

type Config struct {
	Url      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password"`
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
}

func (c *Config) SetTransmission() error {
	conn, err := connection.Db()
	if err != nil {
		return err
	}

	tran, err := conn.Begin()
	if err != nil {
		return err
	}

	query := `
	DELETE FROM frontdesk.transmission;
	`
	_, err = tran.Exec(query)
	if err != nil {
		tran.Rollback()
		return err
	}

	query = `
	INSERT INTO frontdesk.transmission (url, username, password, port, protocol)
	VALUES ($1, $2, $3, $4, $5);
	`
	_, err = tran.Exec(query, c.Url, c.Username, c.Password, c.Port, c.Protocol)
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

	query := `
	SELECT url, username, password, port, protocol
	FROM frontdesk.transmission
	LIMIT 1;
	`

	err = conn.QueryRow(query).Scan(&config.Url, &config.Username, &config.Password, &config.Port, &config.Protocol)
	if err != nil {
		return err
	}

	// building it by hand so a weird char in the password doesnt break everything
	endpoint := &url.URL{
		Scheme: config.Protocol,
		User:   url.UserPassword(config.Username, config.Password),
		Host:   net.JoinHostPort(config.Url, strconv.Itoa(config.Port)),
		Path:   "/transmission/rpc",
	}

	t.Client, err = transmissionrpc.New(endpoint, nil)
	if err != nil {
		fmt.Println("Error:", err)
		return err
	}

	return nil
}

func (t *Transmission) ValidadeVersion() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ok, serverVersion, serverMinimumVersion, err := t.Client.RPCVersion(ctx)
	if err != nil {
		return fmt.Errorf("could not reach transmission: %v", err)
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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	torrents, err := t.Client.TorrentGetAll(ctx)
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
	idsRemove := transmissionrpc.TorrentRemovePayload{ 
		IDs:             ids,
		DeleteLocalData: false,
	} 

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	switch action {
		case "start":
			err = t.Client.TorrentStartIDs(ctx, ids)
		case "stop":
			err = t.Client.TorrentStopIDs(ctx, ids)
		case "remove":
			err = t.Client.TorrentRemove(ctx, idsRemove)
		default:
			return fmt.Errorf("unknown action: %s", action)
	}
	if err != nil {
		return err
	}

	return nil
}