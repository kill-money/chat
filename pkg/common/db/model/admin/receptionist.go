// 二开：接待员邀请码 + 客户绑定 MongoDB 实现
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

// ─── ReceptionistInviteCode ───────────────────────────────────────────────────

func NewReceptionistInviteCode(db *mongo.Database) (admindb.ReceptionistInviteCodeInterface, error) {
	coll := db.Collection(admindb.ReceptionistInviteCode{}.TableName())
	_, err := coll.Indexes().CreateMany(context.Background(), []mongo.IndexModel{
		{Keys: bson.D{{Key: "invite_code", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "user_id", Value: 1}}},
	})
	if err != nil {
		return nil, errs.Wrap(err)
	}
	return &receptionistInviteCode{coll: coll}, nil
}

type receptionistInviteCode struct{ coll *mongo.Collection }

func (r *receptionistInviteCode) TakeByCode(ctx context.Context, code string) (*admindb.ReceptionistInviteCode, error) {
	return mongoutil.FindOne[*admindb.ReceptionistInviteCode](ctx, r.coll, bson.M{"invite_code": code})
}

func (r *receptionistInviteCode) TakeByUserID(ctx context.Context, userID string) (*admindb.ReceptionistInviteCode, error) {
	return mongoutil.FindOne[*admindb.ReceptionistInviteCode](ctx, r.coll, bson.M{"user_id": userID, "status": 1})
}

func (r *receptionistInviteCode) Create(ctx context.Context, rc *admindb.ReceptionistInviteCode) error {
	_, err := r.coll.InsertOne(ctx, rc)
	return err
}

func (r *receptionistInviteCode) UpdateStatus(ctx context.Context, id string, status int32) error {
	_, err := r.coll.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"status": status}})
	return err
}

func (r *receptionistInviteCode) Delete(ctx context.Context, id string) error {
	_, err := r.coll.DeleteOne(ctx, bson.M{"_id": id})
	return err
}

func (r *receptionistInviteCode) Search(ctx context.Context, keyword string, p pagination.Pagination) (int64, []*admindb.ReceptionistInviteCode, error) {
	filter := bson.M{}
	if keyword != "" {
		filter["$or"] = bson.A{
			bson.M{"user_id": bson.M{"$regex": keyword, "$options": "i"}},
			bson.M{"invite_code": bson.M{"$regex": keyword, "$options": "i"}},
		}
	}
	return mongoutil.FindPage[*admindb.ReceptionistInviteCode](ctx, r.coll, filter, p)
}

// ─── CustomerBinding ─────────────────────────────────────────────────────────

func NewCustomerBinding(db *mongo.Database) (admindb.CustomerBindingInterface, error) {
	coll := db.Collection(admindb.CustomerBinding{}.TableName())
	_, err := coll.Indexes().CreateMany(context.Background(), []mongo.IndexModel{
		{Keys: bson.D{{Key: "customer_id", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "receptionist_id", Value: 1}}},
	})
	if err != nil {
		return nil, errs.Wrap(err)
	}
	return &customerBinding{coll: coll}, nil
}

type customerBinding struct{ coll *mongo.Collection }

func (cb *customerBinding) TakeByCustomerID(ctx context.Context, customerID string) (*admindb.CustomerBinding, error) {
	return mongoutil.FindOne[*admindb.CustomerBinding](ctx, cb.coll, bson.M{"customer_id": customerID})
}

func (cb *customerBinding) Create(ctx context.Context, b *admindb.CustomerBinding) error {
	_, err := cb.coll.InsertOne(ctx, b)
	return err
}

func (cb *customerBinding) Update(ctx context.Context, customerID string, receptionistID string, inviteCode string) error {
	_, err := cb.coll.UpdateOne(ctx, bson.M{"customer_id": customerID},
		bson.M{"$set": bson.M{"receptionist_id": receptionistID, "invite_code": inviteCode}})
	return err
}

func (cb *customerBinding) Delete(ctx context.Context, customerID string) error {
	_, err := cb.coll.DeleteOne(ctx, bson.M{"customer_id": customerID})
	return err
}

func (cb *customerBinding) FindByReceptionist(ctx context.Context, receptionistID string) ([]*admindb.CustomerBinding, error) {
	return mongoutil.Find[*admindb.CustomerBinding](ctx, cb.coll, bson.M{"receptionist_id": receptionistID}, options.Find().SetSort(bson.M{"bound_at": -1}))
}

func (cb *customerBinding) CountByReceptionist(ctx context.Context, receptionistID string) (int64, error) {
	return cb.coll.CountDocuments(ctx, bson.M{"receptionist_id": receptionistID})
}
