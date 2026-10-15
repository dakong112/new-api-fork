package controller

import (
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
)

// writeTeamError shows model messages as they are and hides anything else.
func writeTeamError(c *gin.Context, err error) {
	var message *common.Message
	if errors.As(err, &message) {
		common.ApiError(c, message)
		return
	}
	logger.LogError(c, common.LogText("team operation failed for user %d: %s", c.GetInt("id"), err.Error()))
	common.ApiErrorT(c, "Database error, please contact the administrator")
}

// loadTeamMembership resolves :id to a team the caller belongs to. With
// manage set, only the owner and admins pass. Non-members get "Team not
// found" so team ids cannot be probed.
func loadTeamMembership(c *gin.Context, manage bool) (*model.Team, *model.TeamMember, bool) {
	teamId, err := strconv.Atoi(c.Param("id"))
	if err != nil || teamId <= 0 {
		common.ApiErrorT(c, "Team not found")
		return nil, nil, false
	}
	team, member, err := model.GetTeamMembership(teamId, c.GetInt("id"))
	if err != nil {
		writeTeamError(c, err)
		return nil, nil, false
	}
	if manage && member.Role != model.TeamRoleOwner && member.Role != model.TeamRoleAdmin {
		common.ApiErrorT(c, "Only the team owner or an admin can do this")
		return nil, nil, false
	}
	return team, member, true
}

func validTeamLimit(limit int) bool {
	return limit >= 0 && limit <= common.MaxWalletQuota
}

// CreateTeam creates a team owned by the caller, with its wallet account and
// the owner's key.
func CreateTeam(c *gin.Context) {
	var request struct {
		Name       string `json:"name"`
		LimitQuota int    `json:"limit_quota"`
	}
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	name := strings.TrimSpace(request.Name)
	if length := utf8.RuneCountInString(name); length < 1 || length > 64 || !validTeamLimit(request.LimitQuota) {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	team, err := model.CreateTeamTx(c.GetInt("id"), name, request.LimitQuota)
	if err != nil {
		writeTeamError(c, err)
		return
	}
	common.ApiSuccess(c, team)
}

// GetSelfTeams lists the caller's teams with role, team balance and the
// caller's own key usage.
func GetSelfTeams(c *gin.Context) {
	teams, err := model.ListUserTeams(c.GetInt("id"))
	if err != nil {
		writeTeamError(c, err)
		return
	}
	common.ApiSuccess(c, teams)
}

// GetTeam returns a team with its balance. The owner and admins see every
// member; other members see only themselves.
func GetTeam(c *gin.Context) {
	team, member, ok := loadTeamMembership(c, false)
	if !ok {
		return
	}
	balance, err := model.GetUserQuota(team.WalletUserId, true)
	if err != nil {
		writeTeamError(c, err)
		return
	}
	members, err := model.ListTeamMembers(team.Id)
	if err != nil {
		writeTeamError(c, err)
		return
	}
	if member.Role == model.TeamRoleMember {
		visible := make([]model.TeamMemberView, 0, 1)
		for _, m := range members {
			if m.UserId == member.UserId {
				visible = append(visible, m)
			}
		}
		members = visible
	}
	common.ApiSuccess(c, gin.H{"team": team, "role": member.Role, "balance": balance, "members": members})
}

type teamMemberRequest struct {
	Username   string `json:"username"`
	Email      string `json:"email"`
	Role       string `json:"role"`
	LimitQuota *int   `json:"limit_quota"`
}

// AddTeamMember adds a user by username or email. Only the owner may add an
// admin.
func AddTeamMember(c *gin.Context) {
	team, caller, ok := loadTeamMembership(c, true)
	if !ok {
		return
	}
	var request teamMemberRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	request.Username = strings.TrimSpace(request.Username)
	request.Email = strings.TrimSpace(request.Email)
	if request.Role == "" {
		request.Role = model.TeamRoleMember
	}
	limit := 0
	if request.LimitQuota != nil {
		limit = *request.LimitQuota
	}
	if request.Username == "" && request.Email == "" || request.Role != model.TeamRoleMember && request.Role != model.TeamRoleAdmin || !validTeamLimit(limit) {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	if request.Role == model.TeamRoleAdmin && caller.Role != model.TeamRoleOwner {
		common.ApiErrorT(c, "Only the team owner can manage team admins")
		return
	}
	userId, err := model.FindTeamCandidate(request.Username, request.Email)
	if err != nil {
		writeTeamError(c, err)
		return
	}
	member, err := model.AddTeamMember(team.Id, userId, request.Role, limit)
	if err != nil {
		writeTeamError(c, err)
		return
	}
	common.ApiSuccess(c, member)
}

// loadTeamTarget resolves :uid to a member the caller may change: nobody
// changes the owner except the owner, and only the owner touches admins.
func loadTeamTarget(c *gin.Context, team *model.Team, caller *model.TeamMember) (*model.TeamMember, bool) {
	userId, err := strconv.Atoi(c.Param("uid"))
	if err != nil || userId <= 0 {
		common.ApiErrorT(c, "Team member not found")
		return nil, false
	}
	_, target, err := model.GetTeamMembership(team.Id, userId)
	if err != nil {
		common.ApiErrorT(c, "Team member not found")
		return nil, false
	}
	if target.Role != model.TeamRoleMember && caller.Role != model.TeamRoleOwner {
		common.ApiErrorT(c, "Only the team owner can manage team admins")
		return nil, false
	}
	return target, true
}

// UpdateTeamMember changes a member's role and/or spending limit.
func UpdateTeamMember(c *gin.Context) {
	team, caller, ok := loadTeamMembership(c, true)
	if !ok {
		return
	}
	target, ok := loadTeamTarget(c, team, caller)
	if !ok {
		return
	}
	var request teamMemberRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	if request.Role != "" && request.Role != target.Role {
		if target.Role == model.TeamRoleOwner {
			common.ApiError(c, model.ErrTeamOwnerImmutable)
			return
		}
		if request.Role != model.TeamRoleMember && request.Role != model.TeamRoleAdmin {
			common.ApiErrorT(c, "Invalid parameters")
			return
		}
		if request.Role == model.TeamRoleAdmin && caller.Role != model.TeamRoleOwner {
			common.ApiErrorT(c, "Only the team owner can manage team admins")
			return
		}
	}
	if request.LimitQuota != nil && !validTeamLimit(*request.LimitQuota) {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	if request.Role != "" && request.Role != target.Role {
		if err := model.UpdateMemberRole(team.Id, target.UserId, request.Role); err != nil {
			writeTeamError(c, err)
			return
		}
	}
	if request.LimitQuota != nil {
		if err := model.UpdateMemberLimit(team.Id, target.UserId, *request.LimitQuota); err != nil {
			writeTeamError(c, err)
			return
		}
	}
	common.ApiSuccess(c, nil)
}

// DeleteTeamMember removes a member and their key. The owner cannot be removed.
func DeleteTeamMember(c *gin.Context) {
	team, caller, ok := loadTeamMembership(c, true)
	if !ok {
		return
	}
	target, ok := loadTeamTarget(c, team, caller)
	if !ok {
		return
	}
	if err := model.RemoveTeamMember(team.Id, target.UserId); err != nil {
		writeTeamError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// TransferToTeam moves quota from the caller's wallet to the team wallet.
// Any member may contribute.
func TransferToTeam(c *gin.Context) {
	team, _, ok := loadTeamMembership(c, false)
	if !ok {
		return
	}
	var request struct {
		Quota int `json:"quota"`
	}
	if err := common.DecodeJson(c.Request.Body, &request); err != nil || request.Quota <= 0 || request.Quota > common.MaxWalletQuota {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	if err := model.TransferQuotaToTeam(c.GetInt("id"), team.Id, request.Quota); err != nil {
		writeTeamError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"quota": request.Quota})
}

// RedeemToTeam credits a redemption code to the team wallet, like TopUp does
// for the caller's own wallet.
func RedeemToTeam(c *gin.Context) {
	if !operation_setting.IsPaymentComplianceConfirmed() {
		common.ApiErrorT(c, "Payment, redemption, subscription, and invitation reward features are disabled. The administrator must confirm compliance terms before enabling them.")
		return
	}
	team, _, ok := loadTeamMembership(c, false)
	if !ok {
		return
	}
	lock := getTopUpLock(team.WalletUserId)
	if !lock.TryLock() {
		common.ApiErrorT(c, "Top-up is processing, please try again later")
		return
	}
	defer lock.Unlock()
	var request topUpRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	quota, err := model.Redeem(request.Key, team.WalletUserId)
	if err != nil {
		common.ApiErrorT(c, "Redemption failed, please try again later")
		logger.LogError(c, common.LogText("failed to redeem a code for team %d by user %d: %s", team.Id, c.GetInt("id"), err.Error()))
		return
	}
	common.ApiSuccess(c, quota)
}

// GetTeamUsage sums the team wallet's consumption per member key.
func GetTeamUsage(c *gin.Context) {
	team, _, ok := loadTeamMembership(c, true)
	if !ok {
		return
	}
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	usage, err := model.GetTeamUsageByToken(team.WalletUserId, startTimestamp, endTimestamp)
	if err != nil {
		writeTeamError(c, err)
		return
	}
	members, err := model.ListTeamMembers(team.Id)
	if err != nil {
		writeTeamError(c, err)
		return
	}
	memberByToken := make(map[int]model.TeamMemberView, len(members))
	for _, m := range members {
		memberByToken[m.TokenId] = m
	}
	rows := make([]gin.H, 0, len(usage))
	for _, u := range usage {
		m := memberByToken[u.TokenId] // zero for keys of removed members
		rows = append(rows, gin.H{"token_id": u.TokenId, "user_id": m.UserId, "username": m.Username, "quota": u.Quota, "requests": u.Requests})
	}
	common.ApiSuccess(c, rows)
}

// teamForPayment resolves an optional team_id of a payment request to the
// wallet account to credit. Only the owner and admins may top up a team.
func teamForPayment(c *gin.Context, teamId int) (int, bool) {
	if teamId == 0 {
		return c.GetInt("id"), true
	}
	team, member, err := model.GetTeamMembership(teamId, c.GetInt("id"))
	if err != nil {
		paymentError(c, model.ErrTeamNotFound)
		return 0, false
	}
	if member.Role != model.TeamRoleOwner && member.Role != model.TeamRoleAdmin {
		paymentError(c, common.NewMessage("Only the team owner or an admin can do this"))
		return 0, false
	}
	return team.WalletUserId, true
}
