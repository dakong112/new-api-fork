package model

import (
	"strings"
)

// initDefaultVendorMapping assigns a vendor from VendorRules to every enabled
// model that has no metadata in the models table.
func initDefaultVendorMapping(metaMap map[string]*Model, vendorMap map[int]*Vendor, enableAbilities []AbilityWithChannel) {
	rules := currentVendorRules()
	for _, ability := range enableAbilities {
		modelName := ability.Model
		if _, exists := metaMap[modelName]; exists {
			continue
		}
		vendorID := 0
		if index, ok := matchVendorRule(modelName, rules); ok {
			vendorID = getDisplayVendor(rules[index], index, vendorMap)
		}
		metaMap[modelName] = &Model{
			ModelName: modelName,
			VendorID:  vendorID,
			Status:    1,
			NameRule:  NameRuleExact,
		}
	}
}

// getDisplayVendor prefers a database vendor with the rule's name. Otherwise
// the rule is shown as presentation-only vendor -(1001+index): reading pricing
// must never recreate a deleted/merged database record.
func getDisplayVendor(rule VendorRule, index int, vendorMap map[int]*Vendor) int {
	for id, vendor := range vendorMap {
		if strings.EqualFold(vendor.Name, rule.Name) {
			if vendor.Icon == "" {
				vendor.Icon = rule.Icon // display only; vendorMap is never saved
			}
			return id
		}
	}
	id := -(1001 + index)
	vendorMap[id] = &Vendor{Id: id, Name: rule.Name, Status: 1, Icon: rule.Icon}
	return id
}
