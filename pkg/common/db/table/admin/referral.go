// 二开：用户端管理员表与推荐绑定表
package admin

import (
	"context"
	"time"

	"github.com/openimsdk/tools/db/pagination"
)

// ─── UserAdmin ────────────────────────────────────────────────────────────────

// UserAdmin 用户端管理员，持有推荐链接权限、可查看被推荐用户 IP。
type UserAdmin struct {
	ID        string    `bson:"_id"`
	UserID    string    `bson:"user_id"`
	Enabled   bool      `bson:"enabled"`
	CreatedAt time.Time `bson:"created_at"`
}

func (UserAdmin) TableName() string { return "user_admins" }

type UserAdminInterface interface {
	Create(ctx context.Context, u *UserAdmin) error
	Delete(ctx context.Context, userID string) error
	IsAdmin(ctx context.Context, userID string) (bool, error)
	Search(ctx context.Context, keyword string, p pagination.Pagination) (int64, []*UserAdmin, error)
	List(ctx context.Context) ([]*UserAdmin, error)
}

// ─── ReferralBinding ─────────────────────────────────────────────────────────

// ReferralBinding 记录推荐关系：哪个用户管理员推荐了哪个普通用户。
type ReferralBinding struct {
	ID           string    `bson:"_id"`
	AdminID      string    `bson:"admin_id"`
	UserID       string    `bson:"user_id"` // unique — 每个用户只能被推荐一次
	Nickname     string    `bson:"nickname"`
	RegisterIP   string    `bson:"register_ip"`
	RegisterTime time.Time `bson:"register_time"`
}

func (ReferralBinding) TableName() string { return "referral_bindings" }

type ReferralBindingInterface interface {
	Create(ctx context.Context, b *ReferralBinding) error
	TakeByUserID(ctx context.Context, userID string) (*ReferralBinding, error)
	ListByAdmin(ctx context.Context, adminID string) ([]*ReferralBinding, error)
	CountByAdmin(ctx context.Context, adminID string) (int64, error)
}
