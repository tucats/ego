package ast

// This file defines the transaction-control statements, which are otherwise
// unrelated but small enough to group in one file.

// BeginStmt is "BEGIN [DEFERRED | IMMEDIATE | EXCLUSIVE] [TRANSACTION
// [Name]]" (sqlite3) or "BEGIN [TRANSACTION | WORK]" (PostgreSQL). Mode
// holds the sqlite3 transaction-behavior keyword, empty when not written.
// Name holds PostgreSQL/sqlite3's optional trailing transaction name, also
// empty when not written.
type BeginStmt struct {
	BaseStmt
	Mode string
	Name string
}

func (n *BeginStmt) Kind() Kind       { return KindBeginStmt }
func (n *BeginStmt) Children() []Node { return nil }
func (n *BeginStmt) String() string   { return "BeginStmt" }

// CommitStmt is "COMMIT [TRANSACTION | WORK]" (the trailing keyword, if any,
// is not retained since it carries no information).
type CommitStmt struct {
	BaseStmt
}

func (n *CommitStmt) Kind() Kind       { return KindCommitStmt }
func (n *CommitStmt) Children() []Node { return nil }
func (n *CommitStmt) String() string   { return "CommitStmt" }

// RollbackStmt is "ROLLBACK [TRANSACTION | WORK] [TO [SAVEPOINT] To]". To is
// empty for a plain rollback of the whole transaction.
type RollbackStmt struct {
	BaseStmt
	To string
}

func (n *RollbackStmt) Kind() Kind       { return KindRollbackStmt }
func (n *RollbackStmt) Children() []Node { return nil }
func (n *RollbackStmt) String() string   { return "RollbackStmt" }

// SavepointStmt is "SAVEPOINT Name".
type SavepointStmt struct {
	BaseStmt
	Name string
}

func (n *SavepointStmt) Kind() Kind       { return KindSavepointStmt }
func (n *SavepointStmt) Children() []Node { return nil }
func (n *SavepointStmt) String() string   { return "SavepointStmt(" + n.Name + ")" }

// ReleaseStmt is "RELEASE [SAVEPOINT] Name".
type ReleaseStmt struct {
	BaseStmt
	Name string
}

func (n *ReleaseStmt) Kind() Kind       { return KindReleaseStmt }
func (n *ReleaseStmt) Children() []Node { return nil }
func (n *ReleaseStmt) String() string   { return "ReleaseStmt(" + n.Name + ")" }
