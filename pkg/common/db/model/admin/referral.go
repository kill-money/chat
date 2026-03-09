// 二开：用户端管理员 + 推荐绑定 MongoDB 实现
package admin

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	admindb "github.com/openimsdk/chat/pkg/common/db/table/admin"
	"github.com/openimsdk/tools/db/mongoutil"
	"github.com/openimsdk/tools/db/pagination"
	"github.com/openimsdk/tools/errs"
)

// ─── UserAdmin ────────────────────────────────────────────────────────────────

func NewUserAdmin(db *mongo.Database) (admindb.UserAdminInterface, error) {
	coll := db.Collection(admindb.UserAdmin{}.TableName())
	_, err := coll.Indexes().CreateMany(context.Background(), []mongo.IndexModel{
		{Keys: bson.D{{Key: "user_id", Value: 1}}, Options: options.Index().SetUnique(true)},
	})
	if err != nil {
		return nil, errs.Wrap(err)
	}
	return &userAdmin{coll: coll}, nil
}

type userAdmin struct{ coll *mongo.Collection }

func (u *userAdmin) Create(ctx context.Context, admin *admindb.UserAdmin) error {
	_, err := u.coll.InsertOne(ctx, admin)
	return err
}

func (u *userAdmin) Delete(ctx context.Context, userID string) error {
	_, err := u.coll.DeleteOne(ctx, bson.M{"user_id": userID})
	return err
}

func (u *userAdmin) IsAdmin(ctx context.Context, userID string) (bool, error) {
	count, err := u.coll.CountDocuments(ctx, bson.M{"user_id": userID, "enabled": true})
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (u *userAdmin) Search(ctx context.Context, keyword string, p pagination.Pagination) (int64, []*admindb.UserAdmin, error) {
	filter := bson.M{}
	if keyword != "" {
		filter["user_id"] = bson.M{"$regex": keyword, "$options": "i"}
	}
	return mongoutil.FindPage[*admindb.UserAdmin](ctx, u.coll, filter, p)
}

func (u *userAdmin) List(ctx context.Context) ([]*admindb.UserAdmin, error) {
	return mongoutil.Find[*admindb.UserAdmin](ctx, u.coll, bson.M{"enabled": true})
}

// ─── ReferralBinding ─────────────────────────────────────────────────────────

func NewReferralBinding(db *mongo.Database) (admindb.ReferralBindingInterface, error) {
	coll := db.Collection(admindb.ReferralBinding{}.TableName())
	_, err := coll.Indexes().CreateMany(context.Background(), []mongo.IndexModel{
		{Keys: bson.D{{Key: "user_id", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "admin_id", Value: 1}}},
	})
	if err != nil {
		return nil, errs.Wrap(err)
	}
	return &referralBinding{coll: coll}, nil
}

type referralBinding struct{ coll *mongo.Collection }

func (r *referralBinding) Create(ctx context.Context, b *admindb.ReferralBinding) error {
	_, err := r.coll.InsertOne(ctx, b)
	return err
}

func (r *referralBinding) TakeByUserID(ctx context.Context, userID string) (*admindb.ReferralBinding, error) {
	return mongoutil.FindOne[*admindb.ReferralBinding](ctx, r.coll, bson.M{"user_id": userID})
}

func (r *referralBinding) ListByAdmin(ctx context.Context, adminID string) ([]*admindb.ReferralBinding, error) {
	return mongoutil.Find[*admindb.ReferralBinding](ctx, r.coll, bson.M{"admin_id": adminID},
		options.Find().SetSort(bson.M{"register_time": -1}))
}

func (r *referralBinding) CountByAdmin(ctx context.Context, adminID string) (int64, error) {
	return r.coll.CountDocuments(ctx, bson.M{"admin_id": adminID})
}
