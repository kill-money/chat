package admin

import (
	"context"
	"time"

	"github.com/openimsdk/tools/db/pagination"
)

// ReceptionistInviteCode maps to MongoDB collection "receptionist_invite_codes".
type ReceptionistInviteCode struct {
	ID         string    `bson:"_id"`
	UserID     string    `bson:"user_id"`     // receptionist's userID
	InviteCode string    `bson:"invite_code"` // 6-char unique uppercase alphanumeric
	CreatedAt  time.Time `bson:"created_at"`
	Status     int32     `bson:"status"` // 1=enabled 0=disabled
}

func (ReceptionistInviteCode) TableName() string { return "receptionist_invite_codes" }

type ReceptionistInviteCodeInterface interface {
	TakeByCode(ctx context.Context, code string) (*ReceptionistInviteCode, error)
	TakeByUserID(ctx context.Context, userID string) (*ReceptionistInviteCode, error)
	Create(ctx context.Context, r *ReceptionistInviteCode) error
	UpdateStatus(ctx context.Context, id string, status int32) error
	Delete(ctx context.Context, id string) error
	Search(ctx context.Context, keyword string, pagination pagination.Pagination) (int64, []*ReceptionistInviteCode, error)
}

// CustomerBinding maps to MongoDB collection "customer_receptionist_binding".
// customer_id is unique — one customer, one receptionist.
type CustomerBinding struct {
	ID             string    `bson:"_id"`
	CustomerID     string    `bson:"customer_id"`     // unique
	ReceptionistID string    `bson:"receptionist_id"`
	InviteCode     string    `bson:"invite_code"`
	BoundAt        time.Time `bson:"bound_at"`
}

func (CustomerBinding) TableName() string { return "customer_receptionist_binding" }

type CustomerBindingInterface interface {
	TakeByCustomerID(ctx context.Context, customerID string) (*CustomerBinding, error)
	Create(ctx context.Context, b *CustomerBinding) error
	Update(ctx context.Context, customerID string, receptionistID string, inviteCode string) error
	Delete(ctx context.Context, customerID string) error
	FindByReceptionist(ctx context.Context, receptionistID string) ([]*CustomerBinding, error)
	CountByReceptionist(ctx context.Context, receptionistID string) (int64, error)
}
