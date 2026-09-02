package domain

import (
	"context"
)

type Repos interface {
	Assets() AssetRepo
	Classes() ClassRepo
	Logs() LogRepo
	Payments() PaymentRepo
	Settings() SettingsRepo
	Tx(ctx context.Context, fn func(Repos) error) error
	Ping(ctx context.Context) error
	Close() error
}

type AssetFilter struct {
	ClassID int32
	Closed  *bool
	Query   string
	Regex   string
	Page    int
	PerPage int
	Sort    string
}

type EntryFilter struct {
	AssetID int32
	ClassID int32
	From    *Month
	To      *Month
	Query   string
	Regex   string
	Page    int
	PerPage int
	Sort    string
}

type LogFilter = EntryFilter
type PaymentFilter = EntryFilter

func Pagination(page, perPage int) (int, int) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 50
	}
	if perPage > 500 {
		perPage = 500
	}
	return page, perPage
}

type AssetRepo interface {
	List(ctx context.Context, f AssetFilter) ([]Asset, int, error)
	Get(ctx context.Context, id int32) (Asset, error)
	Create(ctx context.Context, a Asset) (Asset, error)
	Update(ctx context.Context, a Asset) (Asset, error)
	SetClosed(ctx context.Context, id int32, closed bool) error
	Delete(ctx context.Context, id int32, force bool) (logsDeleted, paymentsDeleted int64, err error)
}

type ClassRepo interface {
	List(ctx context.Context) ([]Class, error)
	Get(ctx context.Context, id int32) (Class, error)
	Create(ctx context.Context, c Class) (Class, error)
	Update(ctx context.Context, c Class) (Class, error)
	Delete(ctx context.Context, id int32) error
}

type LogRepo interface {
	List(ctx context.Context, f LogFilter) ([]LogEntry, int, error)
	Get(ctx context.Context, id int32) (LogEntry, error)
	Create(ctx context.Context, e LogEntry) (LogEntry, error)
	Update(ctx context.Context, e LogEntry) (LogEntry, error)
	Delete(ctx context.Context, id int32) error
	FindByAssetMonth(ctx context.Context, assetID int32, m Month) (LogEntry, error)
	SeriesRows(ctx context.Context, q SeriesQuery) ([]SeriesRow, error)
}

type PaymentRepo interface {
	List(ctx context.Context, f PaymentFilter) ([]Payment, int, error)
	Get(ctx context.Context, id int32) (Payment, error)
	Create(ctx context.Context, p Payment) (Payment, error)
	Update(ctx context.Context, p Payment) (Payment, error)
	Delete(ctx context.Context, id int32) error
}

// Setting is one full settings row: the key-value pair plus its
// manage-page metadata.
type Setting struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	Description string `json:"description"`
	Display     bool   `json:"display"`
}

type SettingsRepo interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
	Create(ctx context.Context, key, value, description string, display bool) error
	SetDescription(ctx context.Context, key, description string) error
	SetDisplay(ctx context.Context, key string, display bool) error
	Delete(ctx context.Context, key string) error
	All(ctx context.Context) (map[string]string, error)
	List(ctx context.Context) ([]Setting, error)
}

type SeriesQuery struct {
	AssetIDs []int32
	ClassID  int32
	From     *Month
	To       *Month
}

type SeriesRow struct {
	AssetID  int32
	Month    Month
	Deposit  Money
	Value    Money
	Payments Money
	Entries  int
}
