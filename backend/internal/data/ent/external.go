package ent

import (
	"database/sql"
	"entgo.io/ent/dialect"

	entsql "entgo.io/ent/dialect/sql"
)

// Sql exposes the underlying database connection in the ent client
// so that we can use it to perform custom queries.
func (c *Client) Sql() *sql.DB {
	return c.driver.(*entsql.Driver).DB()
}

// SQLDriver exposes transaction-scoped SQL for the custom operation ledger.
func (tx *Tx) SQLDriver() dialect.Driver { return tx.driver }
