// 二开：钱包系统 — MongoDB 表定义与接口
package admin

import (
	"context"
	"time"

	"github.com/openimsdk/tools/db/pagination"
)

// ─── WalletAccount ────────────────────────────────────────────────────────────

// WalletAccount 用户钱包账户，余额单位为分（int64）。
type WalletAccount struct {
	ID        string    `bson:"_id"`
	UserID    string    `bson:"user_id"`  // unique
	Balance   int64     `bson:"balance"`  // 单位：分
	Currency  string    `bson:"currency"` // "CNY"
	CreatedAt time.Time `bson:"created_at"`
	UpdatedAt time.Time `bson:"updated_at"`
}

func (WalletAccount) TableName() string { return "wallet_accounts" }

type WalletAccountInterface interface {
	GetOrCreate(ctx context.Context, userID string) (*WalletAccount, error)
	GetByUserID(ctx context.Context, userID string) (*WalletAccount, error)
	// AdjustBalance atomically adds delta (can be negative) to the user's balance.
	AdjustBalance(ctx context.Context, userID string, delta int64) (*WalletAccount, error)
}

// ─── WalletTransaction ────────────────────────────────────────────────────────

// WalletTransaction 记录每次余额变动。
type WalletTransaction struct {
	ID           string    `bson:"_id"`
	UserID       string    `bson:"user_id"`
	Amount       int64     `bson:"amount"`        // 正=入账，负=扣款（分）
	BalanceAfter int64     `bson:"balance_after"` // 变动后余额（分）
	Note         string    `bson:"note"`          // 管理员备注
	OpAdminID    string    `bson:"op_admin_id"`   // 操作管理员 accountID
	CreatedAt    time.Time `bson:"created_at"`
}

func (WalletTransaction) TableName() string { return "wallet_transactions" }

type WalletTransactionInterface interface {
	Create(ctx context.Context, t *WalletTransaction) error
	SearchByUserID(ctx context.Context, userID string, p pagination.Pagination) (int64, []*WalletTransaction, error)
}

// ─── BankCard ─────────────────────────────────────────────────────────────────

// BankCard 用户绑定的银行卡（仅展示，不做真实扣款）。
type BankCard struct {
	ID         string    `bson:"_id"`
	UserID     string    `bson:"user_id"`
	BankName   string    `bson:"bank_name"`
	CardNumber string    `bson:"card_number"` // 脱敏存储（后4位）
	CardHolder string    `bson:"card_holder"`
	CreatedAt  time.Time `bson:"created_at"`
}

func (BankCard) TableName() string { return "bank_cards" }

type BankCardInterface interface {
	Create(ctx context.Context, b *BankCard) error
	Delete(ctx context.Context, id string, userID string) error
	ListByUserID(ctx context.Context, userID string) ([]*BankCard, error)
}
