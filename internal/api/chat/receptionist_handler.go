// 二开：接待员功能 — 用户端 HTTP handler（chat-api, port 10008）
package chat

import (
	"context"
	"crypto/rand"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	dbutil "github.com/openimsdk/chat/pkg/common/db/dbutil"
	admmodel "github.com/openimsdk/chat/pkg/common/db/model/admin"
	admindb "github.com/openimsdk/chat/pkg/common/db/table/admin"
	chatdb "github.com/openimsdk/chat/pkg/common/db/table/chat"
	"github.com/openimsdk/chat/pkg/common/imapi"
	"github.com/openimsdk/chat/pkg/common/mctx"
	"github.com/openimsdk/tools/apiresp"
	"github.com/openimsdk/tools/db/mongoutil"
	"github.com/openimsdk/tools/errs"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ReceptionistChatHandler user-facing receptionist endpoints + post-registration binding.
type ReceptionistChatHandler struct {
	inviteCodes admindb.ReceptionistInviteCodeInterface
	bindings    admindb.CustomerBindingInterface
	attrColl    *mongo.Collection // attributes collection for user info
	imCaller    imapi.CallerInterface
}

func NewReceptionistChatHandler(cli *mongoutil.Client, imCaller imapi.CallerInterface) (*ReceptionistChatHandler, error) {
	ic, err := admmodel.NewReceptionistInviteCode(cli.GetDB())
	if err != nil {
		return nil, err
	}
	cb, err := admmodel.NewCustomerBinding(cli.GetDB())
	if err != nil {
		return nil, err
	}
	return &ReceptionistChatHandler{
		inviteCodes: ic,
		bindings:    cb,
		attrColl:    cli.GetDB().Collection("attributes"),
		imCaller:    imCaller,
	}, nil
}

// ─── invite code generation ───────────────────────────────────────────────────

const inviteCodeChars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func genInviteCode() string {
	b := make([]byte, 6)
	rand.Read(b)
	for i := range b {
		b[i] = inviteCodeChars[int(b[i])%len(inviteCodeChars)]
	}
	return string(b)
}

// GenInviteCode POST /receptionist/my_code — receptionist gets (or creates) their invite code.
func (h *ReceptionistChatHandler) GenInviteCode(c *gin.Context) {
	userID := mctx.GetOpUserID(c)
	if userID == "" {
		apiresp.GinError(c, errs.ErrNoPermission.WrapMsg("not authenticated"))
		return
	}

	ctx := c.Request.Context()

	// Return existing enabled code if present
	existing, err := h.inviteCodes.TakeByUserID(ctx, userID)
	if err == nil {
		apiresp.GinSuccess(c, gin.H{"inviteCode": existing.InviteCode, "id": existing.ID})
		return
	}
	if !dbutil.IsDBNotFound(err) {
		apiresp.GinError(c, err)
		return
	}

	// Generate a globally unique 6-char code
	var code string
	for i := 0; i < 10; i++ {
		candidate := genInviteCode()
		if _, e := h.inviteCodes.TakeByCode(ctx, candidate); e != nil && dbutil.IsDBNotFound(e) {
			code = candidate
			break
		}
	}
	if code == "" {
		apiresp.GinError(c, errs.ErrInternalServer.WrapMsg("failed to generate unique invite code"))
		return
	}

	entry := &admindb.ReceptionistInviteCode{
		ID:         uuid.New().String(),
		UserID:     userID,
		InviteCode: code,
		CreatedAt:  time.Now(),
		Status:     1,
	}
	if err := h.inviteCodes.Create(ctx, entry); err != nil {
		apiresp.GinError(c, err)
		return
	}
	apiresp.GinSuccess(c, gin.H{"inviteCode": code, "id": entry.ID})
}

// ─── my receptionist ─────────────────────────────────────────────────────────

// MyReceptionist POST /customer/my_receptionist — any logged-in user queries their binding.
func (h *ReceptionistChatHandler) MyReceptionist(c *gin.Context) {
	userID := mctx.GetOpUserID(c)
	if userID == "" {
		apiresp.GinError(c, errs.ErrNoPermission.WrapMsg("not authenticated"))
		return
	}
	ctx := c.Request.Context()

	binding, err := h.bindings.TakeByCustomerID(ctx, userID)
	if err != nil {
		if dbutil.IsDBNotFound(err) {
			apiresp.GinSuccess(c, gin.H{"hasReceptionist": false})
			return
		}
		apiresp.GinError(c, err)
		return
	}

	// Fetch receptionist's nickname + faceURL from attributes collection
	type attrInfo struct {
		Nickname string `bson:"nickname"`
		FaceURL  string `bson:"face_url"`
	}
	var info attrInfo
	_ = h.attrColl.FindOne(ctx, bson.M{"user_id": binding.ReceptionistID}).Decode(&info)

	apiresp.GinSuccess(c, gin.H{
		"hasReceptionist": true,
		"receptionistID":  binding.ReceptionistID,
		"nickname":        info.Nickname,
		"faceURL":         info.FaceURL,
	})
}

// ─── my customers ────────────────────────────────────────────────────────────

type customerItem struct {
	CustomerID string    `json:"customerID"`
	Nickname   string    `json:"nickname"`
	FaceURL    string    `json:"faceURL"`
	LastIP     string    `json:"lastIP"`
	BoundAt    time.Time `json:"boundAt"`
}

// MyCustomers POST /receptionist/my_customers — receptionist lists their bound customers.
func (h *ReceptionistChatHandler) MyCustomers(c *gin.Context) {
	userID := mctx.GetOpUserID(c)
	if userID == "" {
		apiresp.GinError(c, errs.ErrNoPermission.WrapMsg("not authenticated"))
		return
	}
	ctx := c.Request.Context()

	list, err := h.bindings.FindByReceptionist(ctx, userID)
	if err != nil {
		apiresp.GinError(c, err)
		return
	}
	if len(list) == 0 {
		apiresp.GinSuccess(c, gin.H{"total": 0, "customers": []any{}})
		return
	}

	// Batch-fetch attribute info
	ids := make([]any, len(list))
	for i, b := range list {
		ids[i] = b.CustomerID
	}
	type attrRow struct {
		UserID   string `bson:"user_id"`
		Nickname string `bson:"nickname"`
		FaceURL  string `bson:"face_url"`
		LastIP   string `bson:"last_ip"`
	}
	cur, err := h.attrColl.Find(ctx, bson.M{"user_id": bson.M{"$in": ids}}, options.Find().SetProjection(bson.M{"user_id": 1, "nickname": 1, "face_url": 1, "last_ip": 1}))
	attrs := map[string]attrRow{}
	if err == nil {
		defer cur.Close(ctx)
		for cur.Next(ctx) {
			var row attrRow
			if e := cur.Decode(&row); e == nil {
				attrs[row.UserID] = row
			}
		}
	}

	items := make([]customerItem, 0, len(list))
	for _, b := range list {
		a := attrs[b.CustomerID]
		items = append(items, customerItem{
			CustomerID: b.CustomerID,
			Nickname:   a.Nickname,
			FaceURL:    a.FaceURL,
			LastIP:     a.LastIP,
			BoundAt:    b.BoundAt,
		})
	}
	apiresp.GinSuccess(c, gin.H{"total": len(items), "customers": items})
}

// ─── post-registration binding (called from Api.RegisterUser) ─────────────────

// BindAfterRegister checks if inviteCode is a valid receptionist code and creates the binding.
// Returns the receptionistID if bound, "" otherwise. Non-fatal: errors are logged but not returned.
func (h *ReceptionistChatHandler) BindAfterRegister(ctx context.Context, customerID string, inviteCode string, imAdminCtx context.Context) string {
	if strings.TrimSpace(inviteCode) == "" {
		return ""
	}

	entry, err := h.inviteCodes.TakeByCode(ctx, inviteCode)
	if err != nil || entry == nil || entry.Status != 1 {
		return "" // invalid/disabled code — silent fail (not a fatal error)
	}

	// Check if already bound
	if _, err := h.bindings.TakeByCustomerID(ctx, customerID); err == nil {
		return entry.UserID // already bound, return the existing receptionist
	}

	binding := &admindb.CustomerBinding{
		ID:             uuid.New().String(),
		CustomerID:     customerID,
		ReceptionistID: entry.UserID,
		InviteCode:     inviteCode,
		BoundAt:        time.Now(),
	}
	if err := h.bindings.Create(ctx, binding); err != nil {
		return ""
	}

	// Auto add friends bidirectionally via IM API (best-effort)
	_ = h.imCaller.ImportFriend(imAdminCtx, customerID, []string{entry.UserID})
	_ = h.imCaller.ImportFriend(imAdminCtx, entry.UserID, []string{customerID})

	return entry.UserID
}

// ─── attribute helper used in admin receptionist manager ─────────────────────

// GetAttrsByUserIDs fetches attributes for a list of userIDs from the attributes collection.
func (h *ReceptionistChatHandler) getAttrsByUserIDs(ctx context.Context, userIDs []string) map[string]chatdb.Attribute {
	ids := make([]any, len(userIDs))
	for i, id := range userIDs {
		ids[i] = id
	}
	result := map[string]chatdb.Attribute{}
	cur, err := h.attrColl.Find(ctx, bson.M{"user_id": bson.M{"$in": ids}})
	if err != nil {
		return result
	}
	defer cur.Close(ctx)
	for cur.Next(ctx) {
		var a chatdb.Attribute
		if e := cur.Decode(&a); e == nil {
			result[a.UserID] = a
		}
	}
	return result
}
