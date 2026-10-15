package authz

// Dashboard areas behind AdminAuth. Resource and action names equal the
// access token scopes those routes already required ("user:read", ...), so
// existing personal access tokens keep working. The admin role holds every
// action by default; a custom role narrows an administrator to its grants.
var (
	UserRead        = Permission{Resource: "user", Action: ActionRead}
	UserWrite       = Permission{Resource: "user", Action: ActionWrite}
	BillingRead     = Permission{Resource: "billing", Action: ActionRead}
	BillingWrite    = Permission{Resource: "billing", Action: ActionWrite}
	ModelRead       = Permission{Resource: "model", Action: ActionRead}
	ModelWrite      = Permission{Resource: "model", Action: ActionWrite}
	DeploymentRead  = Permission{Resource: "deployment", Action: ActionRead}
	DeploymentWrite = Permission{Resource: "deployment", Action: ActionWrite}
	LogRead         = Permission{Resource: "log", Action: ActionRead}
	RedemptionRead  = Permission{Resource: "redemption", Action: ActionRead}
	RedemptionWrite = Permission{Resource: "redemption", Action: ActionWrite}
	GroupRead       = Permission{Resource: "group", Action: ActionRead}
	GroupWrite      = Permission{Resource: "group", Action: ActionWrite}
)

func init() {
	admin := []string{BuiltInRoleAdmin}
	view := func(description string) ActionDefinition {
		return ActionDefinition{Action: ActionRead, LabelKey: "View", DescriptionKey: description, DefaultRoles: admin}
	}
	edit := func(description string) ActionDefinition {
		return ActionDefinition{Action: ActionWrite, LabelKey: "Edit", DescriptionKey: description, DefaultRoles: admin}
	}
	for _, resource := range []ResourceDefinition{
		{Resource: "user", LabelKey: "Users", Actions: []ActionDefinition{
			view("View users, their sign-in bindings, and permission settings."),
			edit("Create, edit, disable, and delete users, and reset their security settings."),
		}},
		{Resource: "billing", LabelKey: "Billing management", Actions: []ActionDefinition{
			view("View all top-up records, subscription plans, and user subscriptions."),
			edit("Complete top-up orders and manage subscription plans and user subscriptions."),
		}},
		{Resource: "model", LabelKey: "Models", Actions: []ActionDefinition{
			view("View models and vendors, and preview upstream sync."),
			edit("Edit models and vendors, and sync them from upstream."),
		}},
		{Resource: "deployment", LabelKey: "Deployments", Actions: []ActionDefinition{
			view("View deployments and price estimates."),
			edit("Create, edit, and delete deployments, and test connections."),
		}},
		{Resource: "log", LabelKey: "Logs", Actions: []ActionDefinition{
			view("View all users' usage logs, statistics, and tasks."),
		}},
		{Resource: "redemption", LabelKey: "Redemption codes", Actions: []ActionDefinition{
			view("View redemption codes."),
			edit("Create, edit, and delete redemption codes."),
		}},
		{Resource: "group", LabelKey: "Groups", Actions: []ActionDefinition{
			view("View groups and prefill groups."),
			edit("Create, edit, and delete prefill groups."),
		}},
	} {
		RegisterResource(resource)
	}
}
