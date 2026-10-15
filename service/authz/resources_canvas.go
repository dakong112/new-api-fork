package authz

// AppCanvas is the canvas application (infinite). It enforces these codes
// itself after reading them from the SSO grant.
const AppCanvas = "canvas"

const ActionPublish = "publish"

var (
	CanvasTemplateRead    = Permission{Resource: "canvas.template", Action: ActionRead}
	CanvasTemplateWrite   = Permission{Resource: "canvas.template", Action: ActionWrite}
	CanvasTemplatePublish = Permission{Resource: "canvas.template", Action: ActionPublish}
	CanvasUserRead        = Permission{Resource: "canvas.user", Action: ActionRead}
	CanvasOrderRead       = Permission{Resource: "canvas.order", Action: ActionRead}
	CanvasSettingWrite    = Permission{Resource: "canvas.setting", Action: ActionWrite}
	CanvasPromptWrite     = Permission{Resource: "canvas.prompt", Action: ActionWrite}
	CanvasAssetWrite      = Permission{Resource: "canvas.asset", Action: ActionWrite}
)

func init() {
	admin := []string{BuiltInRoleAdmin}
	for _, resource := range []ResourceDefinition{
		{Resource: "canvas.template", LabelKey: "Canvas templates", Actions: []ActionDefinition{
			{Action: ActionRead, LabelKey: "View", DescriptionKey: "View canvas templates, including unpublished ones.", DefaultRoles: admin},
			{Action: ActionWrite, LabelKey: "Edit", DescriptionKey: "Create, edit, and delete canvas templates.", DefaultRoles: admin},
			{Action: ActionPublish, LabelKey: "Publish", DescriptionKey: "Publish canvas templates or take them offline.", DefaultRoles: admin},
		}},
		{Resource: "canvas.user", LabelKey: "Canvas users", Actions: []ActionDefinition{
			{Action: ActionRead, LabelKey: "View", DescriptionKey: "View canvas users and their activity.", DefaultRoles: admin},
		}},
		{Resource: "canvas.order", LabelKey: "Canvas orders", Actions: []ActionDefinition{
			{Action: ActionRead, LabelKey: "View", DescriptionKey: "View canvas orders and template purchases.", DefaultRoles: admin},
		}},
		{Resource: "canvas.setting", LabelKey: "Canvas settings", Actions: []ActionDefinition{
			{Action: ActionWrite, LabelKey: "Edit", DescriptionKey: "Change canvas application settings.", DefaultRoles: admin},
		}},
		{Resource: "canvas.prompt", LabelKey: "Canvas prompts", Actions: []ActionDefinition{
			{Action: ActionWrite, LabelKey: "Edit", DescriptionKey: "Create, edit, and delete canvas prompts.", DefaultRoles: admin},
		}},
		{Resource: "canvas.asset", LabelKey: "Canvas assets", Actions: []ActionDefinition{
			{Action: ActionWrite, LabelKey: "Edit", DescriptionKey: "Manage shared canvas assets.", DefaultRoles: admin},
		}},
	} {
		resource.App = AppCanvas
		RegisterResource(resource)
	}
}
