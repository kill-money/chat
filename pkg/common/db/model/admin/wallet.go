// 二开：钱包系统 — MongoDB 实现
package admin

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/google/uuid"
	admindb "github.com/openimsdk/chat/pkg/common/db/table/admin"
	"github.com/openimsdk/tools/db/mongoutil"
	"github.com/openimsdk/tools/db/pagination"
	"github.com/openimsdk/tools/errs"
)

// ─── WalletAccount ────────────────────────────────────────────────────────────

func NewWalletAccount(db *mongo.Database) (admindb.WalletAccountInterface, error) {
	coll := db.Collection(admindb.WalletAccount{}.TableName())
	_, err := coll.Indexes().CreateMany(context.Background(), []mongo.IndexModel{
		{Keys: bson.D{{Key: "user_id", Value: 1}}, Options: options.Index().SetUnique(true)},
	})
	if err != nil {
		return nil, errs.Wrap(err)
	}
	return &walletAccount{coll: coll}, nil
}

type walletAccount struct{ coll *mongo.Collection }

func (w *walletAccount) GetByUserID(ctx context.Context, userID string) (*admindb.WalletAccount, error) {
	return mongoutil.FindOne[*admindb.WalletAccount](ctx, w.coll, bson.M{"user_id": userID})
}

func (w *walletAccount) GetOrCreate(ctx context.Context, userID string) (*admindb.WalletAccount, error) {
	acc, err := w.GetByUserID(ctx, userID)
	if err == nil {
		return acc, nil
	}
	// Create with zero balance
	now := time.Now()
	acc = &admindb.WalletAccount{
		ID:        uuid.New().String(),
		UserID:    userID,
		Balance:   0,
		Currency:  "CNY",
		CreatedAt: now,
		UpdatedAt: now,
	}
	_, err = w.coll.InsertOne(ctx, acc)
	if err != nil {
		return nil, err
	}
	return acc, nil
}

func (w *walletAccount) AdjustBalance(ctx context.Context, userID string, delta int64) (*admindb.WalletAccount, error) {
	// Ensure account exists
	if _, err := w.GetOrCreate(ctx, userID); err != nil {
		return nil, err
	}
	now := time.Now()
	return mongoutil.FindOneAndUpdate[*admindb.WalletAccount](ctx, w.coll,
		bson.M{"user_id": userID},
		bson.M{
			"$inc": bson.M{"balance": delta},
			"$set": bson.M{"updated_at": now},
		},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	)
}

// ─── WalletTransaction ────────────────────────────────────────────────────────

func NewWalletTransaction(db *mongo.Database) (admindb.WalletTransactionInterface, error) {
	coll := db.Collection(admindb.WalletTransaction{}.TableName())
	_, err := coll.Indexes().CreateMany(context.Background(), []mongo.IndexModel{
		{Keys: bson.D{{Key: "user_id", Value: 1}}},
		{Keys: bson.D{{Key: "created_at", Value: -1}}},
	})
	if err != nil {
		return nil, errs.Wrap(err)
	}
	return &walletTransaction{coll: coll}, nil
}

type walletTransaction struct{ coll *mongo.Collection }

func (t *walletTransaction) Create(ctx context.Context, tx *admindb.WalletTransaction) error {
	_, err := t.coll.InsertOne(ctx, tx)
	return err
}

func (t *walletTransaction) SearchByUserID(ctx context.Context, userID string, p pagination.Pagination) (int64, []*admindb.WalletTransaction, error) {
	return mongoutil.FindPage[*admindb.WalletTransaction](ctx, t.coll,
		bson.M{"user_id": userID},
		p,
		options.Find().SetSort(bson.M{"created_at": -1}),
	)
}

// ─── BankCard ─────────────────────────────────────────────────────────────────

func NewBankCard(db *mongo.Database) (admindb.BankCardInterface, error) {
	coll := db.Collection(admindb.BankCard{}.TableName())
	_, err := coll.Indexes().CreateMany(context.Background(), []mongo.IndexModel{
		{Keys: bson.D{{Key: "user_id", Value: 1}}},
	})
	if err != nil {
		return nil, errs.Wrap(err)
	}
	return &bankCard{coll: coll}, nil
}

type bankCard struct{ coll *mongo.Collection }

func (b *bankCard) Create(ctx context.Context, card *admindb.BankCard) error {
	_, err := b.coll.InsertOne(ctx, card)
	return err
}

func (b *bankCard) Delete(ctx context.Context, id string, userID string) error {
	_, err := b.coll.DeleteOne(ctx, bson.M{"_id": id, "user_id": userID})
	return err
}

func (b *bankCard) ListByUserID(ctx context.Context, userID string) ([]*admindb.BankCard, error) {
	return mongoutil.Find[*admindb.BankCard](ctx, b.coll,
		bson.M{"user_id": userID},
		options.Find().SetSort(bson.M{"created_at": -1}),
	)
}
