package repository

type DB struct{}

func (db *DB) Exec() error {
	return nil
}

type Repositry interface {
	A() error
	B() error
	C() error
}

type Gateway struct {
	db *DB
}

func NewGateway(db *DB) *Gateway {
	return &Gateway{db: db}
}

func (gw *Gateway) A() error {
	return gw.db.Exec()
}

func (gw *Gateway) B() error {
	return gw.db.Exec()
}

func (gw *Gateway) C() error {
	return gw.db.Exec()
}
