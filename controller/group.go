package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

func GetGroups(c *gin.Context) {
	groupNames := make([]string, 0)
	for groupName := range ratio_setting.GetGroupRatioCopy() {
		groupNames = append(groupNames, groupName)
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    groupNames,
	})
}

// GetGroupRatios returns the configured ratio of every group, so admin views
// such as the channel list can show the multiplier next to each group name.
func GetGroupRatios(c *gin.Context) {
	common.ApiSuccess(c, ratio_setting.GetGroupRatioCopy())
}

// GetGroupUsage reports, per group name, how many channels, tokens and users
// reference it, so admins can spot groups with no usable channel.
func GetGroupUsage(c *gin.Context) {
	usage, err := model.GetGroupUsage()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, usage)
}

// RenameGroup renames a group everywhere it is stored (channels, users,
// tokens, subscriptions and group options) in one transaction.
func RenameGroup(c *gin.Context) {
	var req struct {
		OldName string `json:"old_name"`
		NewName string `json:"new_name"`
	}
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	result, err := model.RenameGroup(req.OldName, req.NewName)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "group.rename", map[string]any{
		"old_name": req.OldName,
		"new_name": req.NewName,
		"result":   result,
	})
	common.ApiSuccess(c, result)
}

func GetUserGroups(c *gin.Context) {
	usableGroups := make(map[string]map[string]any)
	userGroup := ""
	userId := c.GetInt("id")
	userGroup, _ = model.GetUserGroup(userId, false)
	userUsableGroups := service.GetUserUsableGroups(userGroup)
	for groupName, _ := range ratio_setting.GetGroupRatioCopy() {
		// UserUsableGroups contains the groups that the user can use
		if desc, ok := userUsableGroups[groupName]; ok {
			usableGroups[groupName] = map[string]any{
				"ratio":           service.GetUserGroupRatio(userGroup, groupName),
				"desc":            desc,
				"fallback_groups": service.GetUserFallbackGroups(userGroup, groupName),
			}
		}
	}
	if _, ok := userUsableGroups["auto"]; ok {
		usableGroups["auto"] = map[string]any{
			"ratio": "自动",
			"desc":  setting.GetUsableGroupDescription("auto"),
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    usableGroups,
	})
}
