// Copyright © 2023 OpenIM open source community. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package chat

import (
	"bytes"
	"encoding/json"
	"io"
	"time"

	chatmw "github.com/openimsdk/chat/internal/api/mw"
	"github.com/openimsdk/chat/internal/api/util"

	"github.com/gin-gonic/gin"
	"github.com/openimsdk/chat/pkg/common/apistruct"
	"github.com/openimsdk/chat/pkg/common/imapi"
	"github.com/openimsdk/chat/pkg/common/mctx"
	"github.com/openimsdk/chat/pkg/eerrs"
	"github.com/openimsdk/chat/pkg/protocol/admin"
	chatpb "github.com/openimsdk/chat/pkg/protocol/chat"
	constantpb "github.com/openimsdk/protocol/constant"
	"github.com/openimsdk/protocol/sdkws"
	"github.com/openimsdk/tools/a2r"
	"github.com/openimsdk/tools/apiresp"
	"github.com/openimsdk/tools/errs"
	"github.com/openimsdk/tools/log"
)

func New(chatClient chatpb.ChatClient, adminClient admin.AdminClient, imApiCaller imapi.CallerInterface, api *util.Api) *Api {
	return &Api{
		Api:         api,
		chatClient:  chatClient,
		adminClient: adminClient,
		imApiCaller: imApiCaller,
	}
}

type Api struct {
	*util.Api
	chatClient  chatpb.ChatClient
	adminClient admin.AdminClient
	imApiCaller imapi.CallerInterface
	Receptionist *ReceptionistChatHandler // 二开：接待员功能，由 start.go 注入
	Referral     *ReferralChatHandler     // 二开：推荐系统，由 start.go 注入
}

// ################## ACCOUNT ##################

func (o *Api) SendVerifyCode(c *gin.Context) {
	req, err := a2r.ParseRequest[chatpb.SendVerifyCodeReq](c)
	if err != nil {
		apiresp.GinError(c, err)
		return
	}
	ip, err := o.GetClientIP(c)
	if err != nil {
		apiresp.GinError(c, err)
		return
	}
	req.Ip = ip
	resp, err := o.chatClient.SendVerifyCode(c, req)
	if err != nil {
		apiresp.GinError(c, err)
		return
	}
	apiresp.GinSuccess(c, resp)
}

func (o *Api) VerifyCode(c *gin.Context) {
	a2r.Call(c, chatpb.ChatClient.VerifyCode, o.chatClient)
}

func (o *Api) RegisterUser(c *gin.Context) {
	// 二开：先读取完整 body，以便提取 downloadReferrer 后再正常解析 proto 字段
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		apiresp.GinError(c, errs.ErrArgs.WrapMsg("read body: "+err.Error()))
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	var extraFields struct {
		DownloadReferrer string `json:"downloadReferrer"`
	}
	_ = json.Unmarshal(bodyBytes, &extraFields)

	req, err := a2r.ParseRequest[chatpb.RegisterUserReq](c)
	if err != nil {
		apiresp.GinError(c, err)
		return
	}
	ip, err := o.GetClientIP(c)
	if err != nil {
		apiresp.GinError(c, err)
		return
	}
	req.Ip = ip

	imToken, err := o.imApiCaller.ImAdminTokenWithDefaultAdmin(c)
	if err != nil {
		apiresp.GinError(c, err)
		return
	}
	apiCtx := mctx.WithApiToken(c, imToken)
	rpcCtx := o.WithAdminUser(c)

	checkResp, err := o.chatClient.CheckUserExist(rpcCtx, &chatpb.CheckUserExistReq{User: req.User})
	if err != nil {
		log.ZDebug(rpcCtx, "Not else", errs.Unwrap(err))
		apiresp.GinError(c, err)
		return
	}
	if checkResp.IsRegistered {
		isUserNotExist, err := o.imApiCaller.AccountCheckSingle(apiCtx, checkResp.Userid)
		if err != nil {
			apiresp.GinError(c, err)
			return
		}
		// if User is  not exist in SDK server. You need delete this user and register new user again.
		if isUserNotExist {
			_, err := o.chatClient.DelUserAccount(rpcCtx, &chatpb.DelUserAccountReq{UserIDs: []string{checkResp.Userid}})
			log.ZDebug(c, "Delete Succsssss", checkResp.Userid)
			if err != nil {
				apiresp.GinError(c, err)
				return
			}
		}
	}

	respRegisterUser, err := o.chatClient.RegisterUser(c, req)
	if err != nil {
		apiresp.GinError(c, err)
		return
	}
	userInfo := &sdkws.UserInfo{
		UserID:     respRegisterUser.UserID,
		Nickname:   req.User.Nickname,
		FaceURL:    req.User.FaceURL,
		CreateTime: time.Now().UnixMilli(),
	}
	err = o.imApiCaller.RegisterUser(apiCtx, []*sdkws.UserInfo{userInfo})
	if err != nil {
		apiresp.GinError(c, err)
		return
	}

	if resp, err := o.adminClient.FindDefaultFriend(rpcCtx, &admin.FindDefaultFriendReq{}); err == nil {
		_ = o.imApiCaller.ImportFriend(apiCtx, respRegisterUser.UserID, resp.UserIDs)
	}
	if resp, err := o.adminClient.FindDefaultGroup(rpcCtx, &admin.FindDefaultGroupReq{}); err == nil {
		_ = o.imApiCaller.InviteToGroup(apiCtx, respRegisterUser.UserID, resp.GroupIDs)
	}
	var resp apistruct.UserRegisterResp
	if req.AutoLogin {
		resp.ImToken, err = o.imApiCaller.GetUserToken(apiCtx, respRegisterUser.UserID, req.Platform)
		if err != nil {
			apiresp.GinError(c, err)
			return
		}
	}
	resp.ChatToken = respRegisterUser.ChatToken
	resp.UserID = respRegisterUser.UserID
	// 二开：若填写了接待员邀请码，执行绑定并返回接待员 userID
	if o.Receptionist != nil && req.InvitationCode != "" {
		resp.ReceptionistID = o.Receptionist.BindAfterRegister(c.Request.Context(), respRegisterUser.UserID, req.InvitationCode, apiCtx)
	}
	// 二开：若携带推荐人 ID（通过下载链接 ?ref= 传入），绑定推荐关系并发送自动消息
	if o.Referral != nil && extraFields.DownloadReferrer != "" {
		nickname := ""
		if req.User != nil {
			nickname = req.User.Nickname
		}
		o.Referral.BindAfterRegister(c.Request.Context(), respRegisterUser.UserID, nickname, req.Ip, extraFields.DownloadReferrer, apiCtx)
	}
	apiresp.GinSuccess(c, &resp)
}

func (o *Api) Login(c *gin.Context) {
	req, err := a2r.ParseRequest[chatpb.LoginReq](c)
	if err != nil {
		apiresp.GinError(c, err)
		return
	}
	ip, err := o.GetClientIP(c)
	if err != nil {
		apiresp.GinError(c, err)
		return
	}
	req.Ip = ip
	// 二开：检查登录锁定（5 次失败后锁定 5 分钟）
	lockKey := req.PhoneNumber
	if req.Account != "" {
		lockKey = req.Account
	} else if req.Email != "" {
		lockKey = req.Email
	}
	if lockKey != "" && chatmw.IsLoginLocked(lockKey) {
		apiresp.GinError(c, eerrs.ErrForbidden.WrapMsg("登录失败次数过多，账号已锁定5分钟"))
		return
	}
	resp, err := o.chatClient.Login(c, req)
	if err != nil {
		// 记录失败次数（密码错误或账号不存在时）
		if lockKey != "" {
			chatmw.RecordLoginFailure(lockKey)
		}
		apiresp.GinError(c, err)
		return
	}
	// 登录成功，重置失败计数
	if lockKey != "" {
		chatmw.ResetLoginFailure(lockKey)
	}
	adminToken, err := o.imApiCaller.ImAdminTokenWithDefaultAdmin(c)
	if err != nil {
		apiresp.GinError(c, err)
		return
	}
	apiCtx := mctx.WithApiToken(c, adminToken)

	imToken, err := o.imApiCaller.GetUserToken(apiCtx, resp.UserID, req.Platform)
	if err != nil {
		apiresp.GinError(c, err)
		return
	}
	apiresp.GinSuccess(c, &apistruct.LoginResp{
		ImToken:     imToken,
		UserID:      resp.UserID,
		ChatToken:   resp.ChatToken,
		AppRole:     resp.AppRole,
		IsUserAdmin: o.Referral != nil && o.Referral.IsUserAdmin(c.Request.Context(), resp.UserID),
	})
}

func (o *Api) ResetPassword(c *gin.Context) {
	a2r.Call(c, chatpb.ChatClient.ResetPassword, o.chatClient)
}

func (o *Api) ChangePassword(c *gin.Context) {
	req, err := a2r.ParseRequest[chatpb.ChangePasswordReq](c)
	if err != nil {
		apiresp.GinError(c, err)
		return
	}
	resp, err := o.chatClient.ChangePassword(c, req)
	if err != nil {
		apiresp.GinError(c, err)
		return
	}

	imToken, err := o.imApiCaller.ImAdminTokenWithDefaultAdmin(c)
	if err != nil {
		apiresp.GinError(c, err)
		return
	}
	err = o.imApiCaller.ForceOffLine(mctx.WithApiToken(c, imToken), req.UserID)
	if err != nil {
		apiresp.GinError(c, err)
		return
	}
	apiresp.GinSuccess(c, resp)
}

// ################## USER ##################

func (o *Api) UpdateUserInfo(c *gin.Context) {
	req, err := a2r.ParseRequest[chatpb.UpdateUserInfoReq](c)
	if err != nil {
		apiresp.GinError(c, err)
		return
	}
	respUpdate, err := o.chatClient.UpdateUserInfo(c, req)
	if err != nil {
		apiresp.GinError(c, err)
		return
	}

	var imToken string
	imToken, err = o.imApiCaller.ImAdminTokenWithDefaultAdmin(c)
	if err != nil {
		apiresp.GinError(c, err)
		return
	}
	var (
		nickName string
		faceURL  string
	)
	if req.Nickname != nil {
		nickName = req.Nickname.Value
	} else {
		nickName = respUpdate.NickName
	}
	if req.FaceURL != nil {
		faceURL = req.FaceURL.Value
	} else {
		faceURL = respUpdate.FaceUrl
	}
	err = o.imApiCaller.UpdateUserInfo(mctx.WithApiToken(c, imToken), req.UserID, nickName, faceURL)
	if err != nil {
		apiresp.GinError(c, err)
		return
	}
	apiresp.GinSuccess(c, apistruct.UpdateUserInfoResp{})
}

func (o *Api) FindUserPublicInfo(c *gin.Context) {
	a2r.Call(c, chatpb.ChatClient.FindUserPublicInfo, o.chatClient)
}

func (o *Api) FindUserFullInfo(c *gin.Context) {
	a2r.Call(c, chatpb.ChatClient.FindUserFullInfo, o.chatClient)
}

func (o *Api) SearchUserFullInfo(c *gin.Context) {
	a2r.Call(c, chatpb.ChatClient.SearchUserFullInfo, o.chatClient)
}

func (o *Api) SearchUserPublicInfo(c *gin.Context) {
	a2r.Call(c, chatpb.ChatClient.SearchUserPublicInfo, o.chatClient)
}

func (o *Api) GetTokenForVideoMeeting(c *gin.Context) {
	a2r.Call(c, chatpb.ChatClient.GetTokenForVideoMeeting, o.chatClient)
}

// 二开：查询指定用户 IP（仅管理员或用户端管理员可调）
func (o *Api) GetUserIPInfo(c *gin.Context) {
	a2r.Call(c, chatpb.ChatClient.GetUserIPInfo, o.chatClient)
}

// ################## APPLET ##################

func (o *Api) FindApplet(c *gin.Context) {
	a2r.Call(c, admin.AdminClient.FindApplet, o.adminClient)
}

// ################## CONFIG ##################

func (o *Api) GetClientConfig(c *gin.Context) {
	a2r.Call(c, admin.AdminClient.GetClientConfig, o.adminClient)
}

// ################## CALLBACK ##################

func (o *Api) OpenIMCallback(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		apiresp.GinError(c, err)
		return
	}
	req := &chatpb.OpenIMCallbackReq{
		Command: c.Query(constantpb.CallbackCommand),
		Body:    string(body),
	}
	if _, err := o.chatClient.OpenIMCallback(c, req); err != nil {
		apiresp.GinError(c, err)
		return
	}
	apiresp.GinSuccess(c, nil)
}

func (o *Api) SearchFriend(c *gin.Context) {
	req, err := a2r.ParseRequest[struct {
		UserID string `json:"userID"`
		chatpb.SearchUserInfoReq
	}](c)
	if err != nil {
		apiresp.GinError(c, err)
		return
	}
	if req.UserID == "" {
		req.UserID = mctx.GetOpUserID(c)
	}
	imToken, err := o.imApiCaller.ImAdminTokenWithDefaultAdmin(c)
	if err != nil {
		apiresp.GinError(c, err)
		return
	}
	userIDs, err := o.imApiCaller.FriendUserIDs(mctx.WithApiToken(c, imToken), req.UserID)
	if err != nil {
		apiresp.GinError(c, err)
		return
	}
	if len(userIDs) == 0 {
		apiresp.GinSuccess(c, &chatpb.SearchUserInfoResp{})
		return
	}
	req.SearchUserInfoReq.UserIDs = userIDs
	resp, err := o.chatClient.SearchUserInfo(c, &req.SearchUserInfoReq)
	if err != nil {
		apiresp.GinError(c, err)
		return
	}
	apiresp.GinSuccess(c, resp)
}

func (o *Api) LatestApplicationVersion(c *gin.Context) {
	a2r.Call(c, admin.AdminClient.LatestApplicationVersion, o.adminClient)
}

func (o *Api) PageApplicationVersion(c *gin.Context) {
	a2r.Call(c, admin.AdminClient.PageApplicationVersion, o.adminClient)
}
