package console_setting

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateContactLinks(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"qq with https url", `[{"icon":"qq","label":"QQ交流群","value":"123456","url":"https://qm.qq.com/q/abc"}]`, false},
		{"value only copies", `[{"icon":"wechat","label":"微信","value":"kid_api"}]`, false},
		{"mailto url", `[{"icon":"email","label":"邮箱","url":"mailto:admin@example.com"}]`, false},
		{"unknown icon", `[{"icon":"weibo","label":"微博","value":"x"}]`, true},
		{"missing label", `[{"icon":"qq","value":"123"}]`, true},
		{"neither value nor url", `[{"icon":"link","label":"官网"}]`, true},
		{"javascript url rejected", `[{"icon":"link","label":"官网","url":"javascript:alert(1)"}]`, true},
		{"empty mailto rejected", `[{"icon":"email","label":"邮箱","url":"mailto:"}]`, true},
		{"script in value rejected", `[{"icon":"qq","label":"QQ","value":"<script>x</script>"}]`, true},
		{"more than 10 items", `[` + strings.TrimSuffix(strings.Repeat(`{"icon":"qq","label":"QQ","value":"1"},`, 11), ",") + `]`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateConsoleSettings(tc.input, "ContactLinks")
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
