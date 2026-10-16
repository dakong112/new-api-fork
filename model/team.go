package model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"gorm.io/gorm"
)

const (
	TeamRoleOwner  = "owner"
	TeamRoleAdmin  = "admin"
	TeamRoleMember = "member"

	// TeamStatusDissolved marks a team whose balance went back to the owner.
	// The row stays so the disabled wallet account is still known as one.
	TeamStatusDissolved = 2

	// maxOwnedTeams bounds how many active wallet accounts one user can create.
	maxOwnedTeams = 20
)

var (
	ErrTeamNotFound          = common.NewMessage("Team not found")
	ErrTeamMemberNotFound    = common.NewMessage("Team member not found")
	ErrTeamMemberExists      = common.NewMessage("User is already a team member")
	ErrTeamMemberUnavailable = common.NewMessage("This user cannot join a team")
	ErrTeamOwnerImmutable    = common.NewMessage("The team owner cannot be changed or removed")
	ErrTeamOwnedLimit        = common.NewMessage("You own too many teams")
	ErrTeamMemberLimit       = common.NewMessage("Team member limit reached")
	ErrTeamInsufficientQuota = common.NewMessage("Insufficient balance")
	ErrTeamBalanceChanged    = common.NewMessage("The team balance changed, please try again")
)

// Team is a shared wallet. Its balance lives on a dedicated user account
// (WalletUserId) so relay billing, top-up and redemption work unchanged; each
// member spends through their own API key under that account, and the key's
// remain_quota is the member's spending cap.
//
// Everything keyed by user id is shared by the team: async tasks (task ids
// from /v1/video, Midjourney, ...) are stored under the wallet account, so
// any member's key can query the task of another member of the same team.
type Team struct {
	Id           int    `json:"id"`
	Name         string `json:"name" gorm:"type:varchar(64)"`
	OwnerId      int    `json:"owner_id" gorm:"index"`
	WalletUserId int    `json:"wallet_user_id" gorm:"uniqueIndex"`
	Status       int    `json:"status" gorm:"type:int;default:1"`
	CreatedAt    int64  `json:"created_at"`
	UpdatedAt    int64  `json:"updated_at"`
}

// TeamMember links a user to a team. TokenId is the member's API key under
// the wallet account; LimitQuota 0 means unlimited.
type TeamMember struct {
	Id         int    `json:"id"`
	TeamId     int    `json:"team_id" gorm:"uniqueIndex:idx_team_members_team_user,priority:1"`
	UserId     int    `json:"user_id" gorm:"uniqueIndex:idx_team_members_team_user,priority:2;index"`
	Role       string `json:"role" gorm:"type:varchar(16)"`
	TokenId    int    `json:"token_id"`
	LimitQuota int    `json:"limit_quota" gorm:"type:bigint;default:0"`
	CreatedAt  int64  `json:"created_at"`
}

// IsTeamWalletUser reports whether userId is a team wallet account, which
// must never sign in. Callers treat an error as "refuse".
func IsTeamWalletUser(userId int) (bool, error) {
	var count int64
	err := DB.Model(&Team{}).Where("wallet_user_id = ?", userId).Count(&count).Error
	return count > 0, err
}

// setTeamMemberKeysEnabledTx disables (or re-enables) the user's team member
// keys when the user account is disabled (or re-enabled), since relay only
// checks the wallet account. Only enabled <-> disabled flips; exhausted keys
// stay exhausted. The cached keys are fenced and dropped first.
func setTeamMemberKeysEnabledTx(tx *gorm.DB, userId int, enabled bool) error {
	from, to := common.TokenStatusEnabled, common.TokenStatusDisabled
	if enabled {
		from, to = to, from
	}
	var tokens []Token
	if err := tx.Where("id IN (?) AND status = ?", tx.Model(&TeamMember{}).Select("token_id").Where("user_id = ?", userId), from).
		Find(&tokens).Error; err != nil {
		return err
	}
	if len(tokens) == 0 {
		return nil
	}
	if err := invalidateTokensCache(tokens); err != nil {
		common.SysLog(common.LogText("failed to invalidate team member key cache for user %d: %s", userId, err.Error()))
	}
	ids := make([]int, 0, len(tokens))
	for _, t := range tokens {
		ids = append(ids, t.Id)
	}
	return tx.Model(&Token{}).Where("id IN ?", ids).Update("status", to).Error
}

// removeUserTeamsTx runs inside user deletion: it deletes the user's
// non-owner memberships and their keys. Owners cannot be deleted while they
// own an active team, so a funded wallet is never orphaned.
func removeUserTeamsTx(tx *gorm.DB, userId int) error {
	var owned int64
	if err := tx.Model(&Team{}).Where("owner_id = ? AND status <> ?", userId, TeamStatusDissolved).Count(&owned).Error; err != nil {
		return err
	}
	if owned > 0 {
		return ErrTeamOwnerImmutable
	}
	var tokens []Token
	if err := tx.Where("id IN (?)", tx.Model(&TeamMember{}).Select("token_id").Where("user_id = ?", userId)).
		Find(&tokens).Error; err != nil {
		return err
	}
	if err := invalidateTokensCache(tokens); err != nil {
		common.SysLog(common.LogText("failed to invalidate team member key cache for user %d: %s", userId, err.Error()))
	}
	if len(tokens) > 0 {
		ids := make([]int, 0, len(tokens))
		for _, t := range tokens {
			ids = append(ids, t.Id)
		}
		if err := tx.Where("id IN ?", ids).Delete(&Token{}).Error; err != nil {
			return err
		}
	}
	return tx.Where("user_id = ?", userId).Delete(&TeamMember{}).Error
}

func teamMemberToken(teamId, walletUserId, userId, limit int) (*Token, error) {
	key, err := common.GenerateKey()
	if err != nil {
		return nil, err
	}
	now := common.GetTimestamp()
	return &Token{
		UserId:         walletUserId,
		Name:           fmt.Sprintf("team-%d-member-%d", teamId, userId),
		Key:            key,
		Status:         common.TokenStatusEnabled,
		CreatedTime:    now,
		AccessedTime:   now,
		ExpiredTime:    -1,
		RemainQuota:    limit,
		UnlimitedQuota: limit == 0,
	}, nil
}

// CreateTeamTx creates, in one transaction, the wallet account (in the owner's
// group, with a random password nobody knows), the team, and the owner's
// membership with an API key capped at ownerLimit (0 = unlimited).
func CreateTeamTx(ownerId int, name string, ownerLimit int) (*Team, error) {
	password, err := common.GenerateKey()
	if err != nil {
		return nil, err
	}
	hashedPassword, err := common.HashAccountPassword(password)
	if err != nil {
		return nil, err
	}
	team := &Team{Name: name, OwnerId: ownerId, Status: common.UserStatusEnabled}
	err = DB.Transaction(func(tx *gorm.DB) error {
		var owner User
		if err := lockForUpdate(tx).First(&owner, ownerId).Error; err != nil {
			return err
		}
		var owned int64
		if err := tx.Model(&Team{}).Where("owner_id = ? AND status <> ?", ownerId, TeamStatusDissolved).Count(&owned).Error; err != nil {
			return err
		}
		if owned >= maxOwnedTeams {
			return ErrTeamOwnedLimit
		}
		wallet := &User{
			Username:    "team_" + strings.ToLower(common.GetRandomString(12)),
			Password:    hashedPassword,
			Role:        common.RoleCommonUser,
			Status:      common.UserStatusEnabled,
			Group:       owner.Group,
			AffCode:     common.GetRandomString(16),
			AuthVersion: 1,
			Remark:      "Team wallet: " + name,
		}
		if err := tx.Create(wallet).Error; err != nil {
			return err
		}
		team.WalletUserId = wallet.Id
		if err := tx.Create(team).Error; err != nil {
			return err
		}
		_, err := addTeamMemberTx(tx, team, ownerId, TeamRoleOwner, ownerLimit)
		return err
	})
	if err != nil {
		return nil, err
	}
	return team, nil
}

func addTeamMemberTx(tx *gorm.DB, team *Team, userId int, role string, limit int) (*TeamMember, error) {
	token, err := teamMemberToken(team.Id, team.WalletUserId, userId, limit)
	if err != nil {
		return nil, err
	}
	if err := tx.Create(token).Error; err != nil {
		return nil, err
	}
	member := &TeamMember{TeamId: team.Id, UserId: userId, Role: role, TokenId: token.Id, LimitQuota: limit, CreatedAt: common.GetTimestamp()}
	if err := tx.Create(member).Error; err != nil {
		return nil, err
	}
	return member, nil
}

// AddTeamMember adds an enabled, non-wallet user with a new API key. The
// wallet row is held so concurrent adds cannot exceed the key limit.
func AddTeamMember(teamId, userId int, role string, limit int) (*TeamMember, error) {
	var member *TeamMember
	err := DB.Transaction(func(tx *gorm.DB) error {
		var team Team
		if err := tx.First(&team, teamId).Error; err != nil {
			return ErrTeamNotFound
		}
		var wallet User
		if err := lockForUpdate(tx).Select("id").First(&wallet, team.WalletUserId).Error; err != nil {
			return err
		}
		var user User
		if err := tx.Select("id", "status").First(&user, userId).Error; err != nil || user.Status != common.UserStatusEnabled {
			return ErrTeamMemberUnavailable
		}
		var wallets int64
		if err := tx.Model(&Team{}).Where("wallet_user_id = ?", userId).Count(&wallets).Error; err != nil {
			return err
		}
		if wallets > 0 {
			return ErrTeamMemberUnavailable
		}
		var existing int64
		if err := tx.Model(&TeamMember{}).Where("team_id = ? AND user_id = ?", teamId, userId).Count(&existing).Error; err != nil {
			return err
		}
		if existing > 0 {
			return ErrTeamMemberExists
		}
		var keys int64
		if err := tx.Model(&Token{}).Where("user_id = ?", team.WalletUserId).Count(&keys).Error; err != nil {
			return err
		}
		if int(keys) >= operation_setting.GetMaxUserTokens() {
			return ErrTeamMemberLimit
		}
		var err error
		member, err = addTeamMemberTx(tx, &team, userId, role, limit)
		return err
	})
	return member, err
}

// GetTeamMembership returns the team and the user's membership in it;
// ErrTeamNotFound when either is missing, so non-members cannot probe ids.
func GetTeamMembership(teamId, userId int) (*Team, *TeamMember, error) {
	var team Team
	if err := DB.First(&team, teamId).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrTeamNotFound
		}
		return nil, nil, err
	}
	var member TeamMember
	if err := DB.Where("team_id = ? AND user_id = ?", teamId, userId).First(&member).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrTeamNotFound
		}
		return nil, nil, err
	}
	return &team, &member, nil
}

// UpdateMemberRole changes a non-owner member's role.
func UpdateMemberRole(teamId, userId int, role string) error {
	result := DB.Model(&TeamMember{}).Where("team_id = ? AND user_id = ? AND role <> ?", teamId, userId, TeamRoleOwner).Update("role", role)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrTeamMemberNotFound
	}
	return nil
}

// UpdateMemberLimit sets the member's lifetime spending cap: the key keeps
// what it already used and may spend limit - used more (never below 0); 0
// makes the key unlimited. An exhausted key comes back to life when the cap
// rises. The cached key is fenced and dropped first, like Token.Update, and
// pending relay deltas still apply to remain/used afterwards.
func UpdateMemberLimit(teamId, userId, limit int) error {
	if limit < 0 || limit > common.MaxWalletQuota {
		return errors.New("invalid team member limit")
	}
	var member TeamMember
	if err := DB.Where("team_id = ? AND user_id = ?", teamId, userId).First(&member).Error; err != nil {
		return ErrTeamMemberNotFound
	}
	var token Token
	if err := DB.First(&token, member.TokenId).Error; err != nil {
		return err
	}
	if err := invalidateTokenCacheForMutation(token.Key); err != nil {
		common.SysLog(common.LogText("failed to invalidate token cache before team limit update: %s", err.Error()))
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&Token{}).Where("id = ?", token.Id).Updates(map[string]any{
			"remain_quota":    gorm.Expr("CASE WHEN used_quota >= ? THEN 0 ELSE ? - used_quota END", limit, limit),
			"unlimited_quota": limit == 0,
			"status":          gorm.Expr("CASE WHEN status = ? THEN ? ELSE status END", common.TokenStatusExhausted, common.TokenStatusEnabled),
		}).Error; err != nil {
			return err
		}
		return tx.Model(&TeamMember{}).Where("id = ?", member.Id).Update("limit_quota", limit).Error
	})
}

// RemoveTeamMember removes a non-owner member and deletes their API key.
func RemoveTeamMember(teamId, userId int) error {
	var member TeamMember
	if err := DB.Where("team_id = ? AND user_id = ?", teamId, userId).First(&member).Error; err != nil {
		return ErrTeamMemberNotFound
	}
	if member.Role == TeamRoleOwner {
		return ErrTeamOwnerImmutable
	}
	var token Token
	if err := DB.First(&token, member.TokenId).Error; err == nil {
		if err := invalidateTokenCacheForMutation(token.Key); err != nil {
			common.SysLog(common.LogText("failed to invalidate token cache before team member removal: %s", err.Error()))
		}
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ?", member.TokenId).Delete(&Token{}).Error; err != nil {
			return err
		}
		return tx.Delete(&member).Error
	})
}

// TransferQuotaToTeam moves quota from a user's wallet to the team wallet in
// one transaction. The debit is a conditional UPDATE (quota >= amount), so
// concurrent transfers can never overdraw; the credit enforces
// common.MaxWalletQuota like a top-up.
// ponytail: like ConsumeSSOQuota, the balance check reads the database row;
// with BATCH_UPDATE_ENABLED, unflushed relay charges are not seen.
func TransferQuotaToTeam(userId, teamId, quota int) error {
	if quota <= 0 || quota > common.MaxWalletQuota {
		return ErrInvalidTopUpQuota
	}
	var team Team
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&team, teamId).Error; err != nil {
			return ErrTeamNotFound
		}
		if team.WalletUserId == userId {
			return ErrTeamMemberUnavailable
		}
		result := tx.Model(&User{}).Where("id = ? AND quota >= ?", userId, quota).Update("quota", gorm.Expr("quota - ?", quota))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrTeamInsufficientQuota
		}
		return creditTopUpQuota(tx, team.WalletUserId, quota, nil)
	})
	if err != nil {
		return err
	}
	if err := cacheDecrUserQuota(userId, int64(quota)); err != nil {
		common.SysLog(common.LogText("failed to decrease user quota cache after team transfer: %s", err.Error()))
	}
	syncCreditUserQuotaCache(team.WalletUserId, quota, common.LogText("team transfer"))
	username, _ := GetUsernameById(userId, false)
	RecordLog(userId, LogTypeSystem, common.NewMessage("Transferred {{quota}} to team {{team}}", map[string]any{"quota": logger.FormatQuota(quota), "team": team.Name}))
	RecordLog(team.WalletUserId, LogTypeTopup, common.NewMessage("Received {{quota}} transferred from {{username}}", map[string]any{"quota": logger.FormatQuota(quota), "username": username}))
	return nil
}

// DissolveTeam ends a team in one transaction: the wallet balance goes back
// to the owner, every key of the wallet account and every membership is
// deleted, the wallet account is disabled (kept for its logs) and the team is
// marked dissolved. The team and wallet rows are locked, and the wallet is
// zeroed only if it still holds the balance that was read, so a concurrent
// debit or credit makes the call fail instead of returning a stale amount.
// ponytail: like TransferQuotaToTeam, unflushed BATCH_UPDATE relay charges are
// not seen; they land on the disabled wallet afterwards.
func DissolveTeam(teamId, ownerId int) (int, error) {
	var team Team
	var tokens []Token
	quota := 0
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := lockForUpdate(tx).First(&team, teamId).Error; err != nil || team.Status == TeamStatusDissolved || team.OwnerId != ownerId {
			return ErrTeamNotFound
		}
		var wallet User
		if err := lockForUpdate(tx).Select("id", "quota").First(&wallet, team.WalletUserId).Error; err != nil {
			return err
		}
		if wallet.Quota > 0 {
			quota = wallet.Quota
			result := tx.Model(&User{}).Where("id = ? AND quota = ?", wallet.Id, quota).Update("quota", 0)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return ErrTeamBalanceChanged
			}
			if err := creditTopUpQuota(tx, ownerId, quota, nil); err != nil {
				return err
			}
		}
		if err := tx.Model(&User{}).Where("id = ?", wallet.Id).Update("status", common.UserStatusDisabled).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", wallet.Id).Find(&tokens).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", wallet.Id).Delete(&Token{}).Error; err != nil {
			return err
		}
		if err := tx.Where("team_id = ?", team.Id).Delete(&TeamMember{}).Error; err != nil {
			return err
		}
		return tx.Model(&Team{}).Where("id = ?", team.Id).Update("status", TeamStatusDissolved).Error
	})
	if err != nil {
		return 0, err
	}
	if err := invalidateTokensCache(tokens); err != nil {
		common.SysLog(common.LogText("failed to invalidate team key cache after dissolving team %d: %s", team.Id, err.Error()))
	}
	if err := invalidateUserCache(team.WalletUserId); err != nil {
		common.SysLog(common.LogText("failed to invalidate team wallet cache after dissolving team %d: %s", team.Id, err.Error()))
	}
	syncCreditUserQuotaCache(ownerId, quota, common.LogText("team dissolve"))
	if quota > 0 {
		RecordLog(ownerId, LogTypeSystem, common.NewMessage("Received {{quota}} from dissolved team {{team}}", map[string]any{"quota": logger.FormatQuota(quota), "team": team.Name}))
		RecordLog(team.WalletUserId, LogTypeSystem, common.NewMessage("Team dissolved, {{quota}} returned to the owner", map[string]any{"quota": logger.FormatQuota(quota)}))
	}
	return quota, nil
}

// topUpRecipientTx returns the user a paid top-up order of userId credits:
// the team owner when userId is the wallet of a dissolved team (an order
// created before DissolveTeam and paid after), otherwise userId itself. The
// team row is locked like DissolveTeam does, so a payment racing a dissolve
// either lands on the wallet before its balance is returned or goes to the
// owner, never onto the disabled wallet.
func topUpRecipientTx(tx *gorm.DB, userId int) (int, error) {
	var teams []Team
	if err := lockForUpdate(tx).Select("id", "owner_id", "status").Where("wallet_user_id = ?", userId).Limit(1).Find(&teams).Error; err != nil {
		return 0, err
	}
	if len(teams) == 0 || teams[0].Status != TeamStatusDissolved {
		return userId, nil
	}
	common.SysLog(common.LogText("top-up for wallet %d of dissolved team %d credited to owner %d", userId, teams[0].Id, teams[0].OwnerId))
	return teams[0].OwnerId, nil
}

// UserTeam is a team as seen by one of its members.
type UserTeam struct {
	Team
	Role           string `json:"role"`
	LimitQuota     int    `json:"limit_quota"`
	Balance        int    `json:"balance"`
	KeyRemainQuota int    `json:"key_remain_quota"`
	KeyUsedQuota   int    `json:"key_used_quota"`
}

// ListUserTeams lists the teams userId belongs to with the team balance and
// the user's own key usage.
func ListUserTeams(userId int) ([]UserTeam, error) {
	teams := make([]UserTeam, 0)
	err := DB.Table("team_members").
		Select("teams.*, team_members.role, team_members.limit_quota, users.quota AS balance, tokens.remain_quota AS key_remain_quota, tokens.used_quota AS key_used_quota").
		Joins("JOIN teams ON teams.id = team_members.team_id").
		Joins("JOIN users ON users.id = teams.wallet_user_id").
		Joins("LEFT JOIN tokens ON tokens.id = team_members.token_id").
		Where("team_members.user_id = ?", userId).
		Order("teams.id").
		Scan(&teams).Error
	return teams, err
}

// TeamMemberView is a member row with the user's name and key usage.
type TeamMemberView struct {
	TeamMember
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	RemainQuota int    `json:"remain_quota"`
	UsedQuota   int    `json:"used_quota"`
}

func ListTeamMembers(teamId int) ([]TeamMemberView, error) {
	members := make([]TeamMemberView, 0)
	err := DB.Table("team_members").
		Select("team_members.*, users.username, users.display_name, tokens.remain_quota, tokens.used_quota").
		Joins("JOIN users ON users.id = team_members.user_id").
		Joins("LEFT JOIN tokens ON tokens.id = team_members.token_id").
		Where("team_members.team_id = ?", teamId).
		Order("team_members.id").
		Scan(&members).Error
	return members, err
}

// TeamTokenUsage is the consumption of one key of a team wallet.
type TeamTokenUsage struct {
	TokenId  int   `json:"token_id"`
	Quota    int64 `json:"quota"`
	Requests int64 `json:"requests"`
}

// GetTeamUsageByToken sums the wallet account's consume logs per key. Logs
// live in LOG_DB, which may be a separate database.
func GetTeamUsageByToken(walletUserId int, startTimestamp, endTimestamp int64) ([]TeamTokenUsage, error) {
	usage := make([]TeamTokenUsage, 0)
	tx := LOG_DB.Table("logs").
		Select("token_id, COALESCE(SUM(quota), 0) AS quota, COUNT(*) AS requests").
		Where("user_id = ? AND type = ?", walletUserId, LogTypeConsume)
	if startTimestamp > 0 {
		tx = tx.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp > 0 {
		tx = tx.Where("created_at <= ?", endTimestamp)
	}
	err := tx.Group("token_id").Order("token_id").Scan(&usage).Error
	return usage, err
}

// GetTeamMemberToken returns the member's API key for the team.
func GetTeamMemberToken(teamId, userId int) (*Token, error) {
	_, member, err := GetTeamMembership(teamId, userId)
	if err != nil {
		return nil, err
	}
	var token Token
	if err := DB.First(&token, member.TokenId).Error; err != nil {
		return nil, err
	}
	return &token, nil
}

// FindTeamCandidate resolves a username or an email to a user id.
func FindTeamCandidate(username, email string) (int, error) {
	if username != "" {
		var user User
		if err := DB.Select("id").Where("username = ?", username).First(&user).Error; err != nil {
			return 0, ErrTeamMemberUnavailable
		}
		return user.Id, nil
	}
	user, err := GetUniqueUserByEmail(email)
	if err != nil {
		return 0, ErrTeamMemberUnavailable
	}
	return user.Id, nil
}
