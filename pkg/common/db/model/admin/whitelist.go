// 二开：白名单 MongoDB 实现
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

func NewWhitelistUser(db *mongo.Database) (admindb.WhitelistInterface, error) {
	coll := db.Collection("whitelist_users")
	_, err := coll.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys:    bson.D{{Key: "identifier", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return nil, errs.Wrap(err)
	}
	return &WhitelistUser{coll: coll}, nil
}

type WhitelistUser struct {
	coll *mongo.Collection
}

func (o *WhitelistUser) TakeByIdentifier(ctx context.Context, identifier string) (*admindb.WhitelistUser, error) {
	return mongoutil.FindOne[*admindb.WhitelistUser](ctx, o.coll, bson.M{"identifier": identifier})
}

func (o *WhitelistUser) TakeByID(ctx context.Context, id string) (*admindb.WhitelistUser, error) {
	return mongoutil.FindOne[*admindb.WhitelistUser](ctx, o.coll, bson.M{"_id": id})
}

func (o *WhitelistUser) Create(ctx context.Context, users []*admindb.WhitelistUser) error {
	return mongoutil.InsertMany(ctx, o.coll, users)
}

func (o *WhitelistUser) Update(ctx context.Context, id string, update map[string]any) error {
	return mongoutil.UpdateOne(ctx, o.coll, bson.M{"_id": id}, bson.M{"$set": update}, false)
}

func (o *WhitelistUser) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return mongoutil.DeleteMany(ctx, o.coll, bson.M{"_id": bson.M{"$in": ids}})
}

func (o *WhitelistUser) Search(ctx context.Context, keyword string, status int32, pagination pagination.Pagination) (int64, []*admindb.WhitelistUser, error) {
	filter := bson.M{}
	if status >= 0 {
		filter["status"] = status
	}
	if keyword != "" {
		filter["$or"] = []bson.M{
			{"identifier": bson.M{"$regex": keyword, "$options": "i"}},
			{"remark": bson.M{"$regex": keyword, "$options": "i"}},
			{"role": bson.M{"$regex": keyword, "$options": "i"}},
		}
	}
	return mongoutil.FindPage[*admindb.WhitelistUser](ctx, o.coll, filter, pagination)
}
