package paychecker

import (
	"time"

	"github.com/glgaspar/front_desk/connection"
)

type Bill struct {
	Id          int        `json:"id" db:"id"`                   // Bill Id
	Description string     `json:"description" db:"description"` // What you are paying
	ExpDay      int        `json:"expDay" db:"expDay"`           // Expiration day
	Path        string     `json:"path" db:"path"`               // Where to find the files
	LastDate    time.Time `json:"lastDate" db:"lastDate"`       // Date of last payment
	Track       bool      `json:"track" db:"track"`             // Is that bill active?
}

func (b *Bill) GetAllBills() ([]Bill, error) {
	var bills []Bill
	conn, err := connection.Db()
	if err != nil {
		return bills, err
	}
	defer conn.Close()
	query := `
	select 
		id, description, expDay, lastDate, path, track
	from paychecker.bills
	`

	result, err := conn.Query(query)
	if err != nil {
		return bills, err
	}

	for result.Next() {
		result.Scan(&b.Id, &b.Description, &b.ExpDay, &b.LastDate, &b.Path, &b.Track)
		bills = append(bills, *b)
	}

	return bills, nil
}

func (b *Bill) CreateBill() (Bill, error) {
	conn, err := connection.Db()
	if err != nil {
		return Bill{}, err
	}
	defer conn.Close()

	query := `
	insert into paychecker.bills (description,expDay,path,track)
	values ($1, $2, $3, $4)
	RETURNING *`
	newBill, err := conn.Query(query, b.Description, b.ExpDay, b.Path, b.Track)
	if err != nil {
		return Bill{}, err
	}

	for newBill.Next() {
		newBill.Scan(&b.Id, &b.Description, &b.ExpDay, &b.LastDate, &b.Path, &b.Track)
	}
	return *b, nil
}

func (b *Bill) FlipTrack() error {
	conn, err := connection.Db()
	if err != nil {
		return err
	}
	defer conn.Close()

	query := `
	update paychecker.bills
	set 
		track = not track
	where 
		id = $5 
	returning *
		`
	newBill, err := conn.Query(query, b.Id)
	if err != nil {
		return err
	}
	for newBill.Next() {
		newBill.Scan(&b.Id, &b.Description, &b.ExpDay, &b.LastDate, &b.Path, &b.Track)
	}

	return nil
}

func (b *Bill) PayBill() error {
	conn, err := connection.Db()
	if err != nil {
		return err
	}
	defer conn.Close()

	query := `
	update paychecker.bills
	set
		lastDate $1
	where 
		id = $2
	`
	_, err = conn.Query(query, time.Now(), b.Id)
	if err != nil {
		return err
	}

	return nil
}
