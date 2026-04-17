package graphroute

import (
	"strings"
	"testing"

	"github.com/gastownhall/gascity/internal/beads"
	"github.com/gastownhall/gascity/internal/config"
	"github.com/gastownhall/gascity/internal/formula"
	"github.com/gastownhall/gascity/internal/session"
)

func TestIsCompiledGraphWorkflow(t *testing.T) {
	t.Run("nil recipe", func(t *testing.T) {
		if IsCompiledGraphWorkflow(nil) {
			t.Error("expected false for nil recipe")
		}
	})
	t.Run("empty steps", func(t *testing.T) {
		if IsCompiledGraphWorkflow(&formula.Recipe{}) {
			t.Error("expected false for empty steps")
		}
	})
	t.Run("graph workflow", func(t *testing.T) {
		r := &formula.Recipe{
			Steps: []formula.RecipeStep{{
				Metadata: map[string]string{
					"gc.kind":             "workflow",
					"gc.formula_contract": "graph.v2",
				},
			}},
		}
		if !IsCompiledGraphWorkflow(r) {
			t.Error("expected true for graph.v2 workflow")
		}
	})
}

func TestIsControlDispatcherKind(t *testing.T) {
	for _, kind := range []string{"check", "fanout", "retry-eval", "scope-check", "workflow-finalize", "retry", "ralph"} {
		if !IsControlDispatcherKind(kind) {
			t.Errorf("expected true for %q", kind)
		}
	}
	if IsControlDispatcherKind("task") {
		t.Error("expected false for task")
	}
}

func TestGraphRouteRigContext(t *testing.T) {
	if got := GraphRouteRigContext("myrig/worker"); got != "myrig" {
		t.Errorf("got %q, want myrig", got)
	}
	if got := GraphRouteRigContext("mayor"); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestGraphWorkflowRouteVars(t *testing.T) {
	dflt := "default-val"
	r := &formula.Recipe{
		Vars: map[string]*formula.VarDef{
			"base": {Default: &dflt},
		},
	}
	got := GraphWorkflowRouteVars(r, map[string]string{"override": "yes"})
	if got["base"] != "default-val" {
		t.Errorf("base = %q, want default-val", got["base"])
	}
	if got["override"] != "yes" {
		t.Errorf("override = %q, want yes", got["override"])
	}
}

func intPtr(v int) *int { return &v }

func TestApplyGraphRouting_LegacyStampsSteps(t *testing.T) {
	// Legacy [[steps]] recipe: every step should inherit gc.routed_to from
	// the wisp root's target so tier-3 work queries can find them.
	r := &formula.Recipe{
		Steps: []formula.RecipeStep{
			{ID: "dry", Metadata: map[string]string{"gc.kind": "task"}},
			{ID: "wet", Metadata: map[string]string{"gc.kind": "task"}},
			{ID: "combine"},
		},
	}
	a := config.Agent{Name: "worker", MaxActiveSessions: intPtr(1)}
	err := ApplyGraphRouting(r, &a, "myrig/worker", nil, "", "", "", "", nil, "city", &config.City{}, Deps{})
	if err != nil {
		t.Fatalf("ApplyGraphRouting: %v", err)
	}
	for i, step := range r.Steps {
		if got := step.Metadata["gc.routed_to"]; got != "myrig/worker" {
			t.Errorf("step[%d] gc.routed_to = %q, want myrig/worker", i, got)
		}
	}
}

func TestApplyGraphRouting_LegacyPreservesExplicitRoute(t *testing.T) {
	// A step with a pre-set gc.routed_to must not be overwritten.
	r := &formula.Recipe{
		Steps: []formula.RecipeStep{
			{ID: "default"},
			{ID: "explicit", Metadata: map[string]string{"gc.routed_to": "other/agent"}},
		},
	}
	err := ApplyGraphRouting(r, nil, "myrig/worker", nil, "", "", "", "", nil, "city", nil, Deps{})
	if err != nil {
		t.Fatalf("ApplyGraphRouting: %v", err)
	}
	if got := r.Steps[0].Metadata["gc.routed_to"]; got != "myrig/worker" {
		t.Errorf("default step gc.routed_to = %q, want myrig/worker", got)
	}
	if got := r.Steps[1].Metadata["gc.routed_to"]; got != "other/agent" {
		t.Errorf("explicit step gc.routed_to = %q, want preserved other/agent", got)
	}
}

func TestApplyGraphRouting_LegacyEmptyRouteNoOp(t *testing.T) {
	// Controller probing without a concrete agent target: do not stamp
	// an empty string on steps.
	r := &formula.Recipe{
		Steps: []formula.RecipeStep{
			{ID: "step", Metadata: map[string]string{"gc.kind": "task"}},
		},
	}
	err := ApplyGraphRouting(r, nil, "", nil, "", "", "", "", nil, "city", nil, Deps{})
	if err != nil {
		t.Fatalf("ApplyGraphRouting: %v", err)
	}
	if got, ok := r.Steps[0].Metadata["gc.routed_to"]; ok {
		t.Errorf("gc.routed_to = %q, want unset", got)
	}
}

func TestApplyGraphRouting_LegacyNilMetadata(t *testing.T) {
	// A step with no metadata map should get one created and stamped.
	r := &formula.Recipe{
		Steps: []formula.RecipeStep{{ID: "step"}},
	}
	err := ApplyGraphRouting(r, nil, "myrig/worker", nil, "", "", "", "", nil, "city", nil, Deps{})
	if err != nil {
		t.Fatalf("ApplyGraphRouting: %v", err)
	}
	if got := r.Steps[0].Metadata["gc.routed_to"]; got != "myrig/worker" {
		t.Errorf("gc.routed_to = %q, want myrig/worker", got)
	}
}

func TestApplyGraphRouting_NilRecipe(t *testing.T) {
	// Nil recipe is a no-op (matches existing defensive guard).
	if err := ApplyGraphRouting(nil, nil, "worker", nil, "", "", "", "", nil, "city", nil, Deps{}); err != nil {
		t.Fatalf("ApplyGraphRouting(nil) = %v, want nil", err)
	}
}

type testAgentResolver struct{}

func (testAgentResolver) ResolveAgent(cfg *config.City, name, _ string) (config.Agent, bool) {
	for _, a := range cfg.Agents {
		if a.QualifiedName() == name || a.Name == name {
			return a, true
		}
	}
	return config.Agent{}, false
}

func TestDecorateGraphWorkflowRecipe_SetsRootMetadata(t *testing.T) {
	cfg := &config.City{Agents: []config.Agent{
		{Name: "mayor", MaxActiveSessions: intPtr(1)},
		{Name: "control-dispatcher", MaxActiveSessions: intPtr(1)},
	}}
	r := &formula.Recipe{
		Name: "wf-test",
		Steps: []formula.RecipeStep{
			{ID: "wf-test.root", IsRoot: true, Metadata: map[string]string{
				"gc.kind": "workflow", "gc.formula_contract": "graph.v2",
			}},
			{ID: "wf-test.work", Metadata: map[string]string{}},
		},
	}
	deps := Deps{Resolver: testAgentResolver{}}
	err := DecorateGraphWorkflowRecipe(r, nil, "src-1", "city", "test-city", "city:test", "mayor", "test--mayor", nil, "test-city", cfg, deps)
	if err != nil {
		t.Fatalf("DecorateGraphWorkflowRecipe: %v", err)
	}
	root := r.Steps[0]
	if root.Metadata["gc.run_target"] != "mayor" {
		t.Errorf("root gc.run_target = %q, want mayor", root.Metadata["gc.run_target"])
	}
	if root.Metadata["gc.source_bead_id"] != "src-1" {
		t.Errorf("root gc.source_bead_id = %q, want src-1", root.Metadata["gc.source_bead_id"])
	}
	if root.Metadata["gc.scope_kind"] != "city" {
		t.Errorf("root gc.scope_kind = %q, want city", root.Metadata["gc.scope_kind"])
	}
	// Work step should have gc.routed_to set.
	work := r.Steps[1]
	if work.Metadata["gc.routed_to"] != "mayor" {
		t.Errorf("work gc.routed_to = %q, want mayor", work.Metadata["gc.routed_to"])
	}
}

func TestDecorateGraphWorkflowRecipe_NilRecipe(t *testing.T) {
	err := DecorateGraphWorkflowRecipe(nil, nil, "", "", "", "", "", "", nil, "", nil, Deps{})
	if err == nil {
		t.Error("expected error for nil recipe")
	}
}

func TestResolveGraphStepBinding_CycleDetection(t *testing.T) {
	// Step A has kind "check" with dep on B, B has kind "check" with dep on A.
	// This creates a routing cycle.
	stepA := &formula.RecipeStep{ID: "A", Metadata: map[string]string{"gc.kind": "check"}}
	stepB := &formula.RecipeStep{ID: "B", Metadata: map[string]string{"gc.kind": "check"}}
	stepByID := map[string]*formula.RecipeStep{"A": stepA, "B": stepB}
	depsByStep := map[string][]string{"A": {"B"}, "B": {"A"}}
	cache := make(map[string]GraphRouteBinding)
	resolving := make(map[string]bool)
	fallback := GraphRouteBinding{QualifiedName: "default"}

	_, err := ResolveGraphStepBinding("A", stepByID, nil, depsByStep, cache, resolving, fallback, "", nil, "", nil, Deps{})
	if err == nil {
		t.Error("expected cycle detection error")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("error = %q, want cycle mention", err.Error())
	}
}

func TestResolveGraphStepBinding_AssigneeTemplateTargetRejected(t *testing.T) {
	cfg := &config.City{
		Workspace: config.Workspace{Name: "test-city"},
		Agents: []config.Agent{
			{Name: "worker", Dir: "frontend", MaxActiveSessions: intPtr(1)},
		},
	}
	stepByID := map[string]*formula.RecipeStep{
		"demo.work": {
			ID:       "demo.work",
			Title:    "Work",
			Assignee: "worker",
		},
	}
	cache := make(map[string]GraphRouteBinding)
	resolving := make(map[string]bool)

	_, err := ResolveGraphStepBinding("demo.work", stepByID, nil, nil, cache, resolving, GraphRouteBinding{}, "frontend", beads.NewMemStore(), cfg.Workspace.Name, cfg, Deps{Resolver: testAgentResolver{}})
	if err == nil {
		t.Fatal("ResolveGraphStepBinding unexpectedly succeeded for template assignee")
	}
	if !strings.Contains(err.Error(), "use gc.run_target for config routing") {
		t.Fatalf("ResolveGraphStepBinding error = %q, want gc.run_target guidance", err)
	}
}

func TestResolveGraphStepBinding_AssigneeConcreteSessionBeatsTemplateCollision(t *testing.T) {
	cfg := &config.City{
		Workspace: config.Workspace{Name: "test-city"},
		Agents: []config.Agent{
			{Name: "worker", Dir: "frontend", MaxActiveSessions: intPtr(1)},
		},
	}
	store := beads.NewMemStoreFrom(1, []beads.Bead{{
		ID:     "worker",
		Type:   session.BeadType,
		Status: "open",
		Labels: []string{session.LabelSession},
		Metadata: map[string]string{
			"session_name": "s-frontend-worker",
			"alias":        "frontend/worker-live",
			"template":     "frontend/worker",
			"state":        "active",
		},
	}}, nil)
	stepByID := map[string]*formula.RecipeStep{
		"demo.work": {
			ID:       "demo.work",
			Title:    "Work",
			Assignee: "worker",
		},
	}
	cache := make(map[string]GraphRouteBinding)
	resolving := make(map[string]bool)

	binding, err := ResolveGraphStepBinding("demo.work", stepByID, nil, nil, cache, resolving, GraphRouteBinding{}, "frontend", store, cfg.Workspace.Name, cfg, Deps{Resolver: testAgentResolver{}})
	if err != nil {
		t.Fatalf("ResolveGraphStepBinding: %v", err)
	}
	if binding.DirectSessionID != "worker" {
		t.Fatalf("DirectSessionID = %q, want worker", binding.DirectSessionID)
	}
}

func TestControlDispatcherBinding_NilConfig(t *testing.T) {
	_, err := ControlDispatcherBinding(nil, "city", nil, "", Deps{})
	if err == nil {
		t.Error("expected error for nil config")
	}
}

func TestControlDispatcherBinding_NilResolver(t *testing.T) {
	cfg := &config.City{}
	_, err := ControlDispatcherBinding(nil, "city", cfg, "", Deps{})
	if err == nil {
		t.Error("expected error for nil resolver")
	}
}

func TestWorkflowExecutionRoute(t *testing.T) {
	b := beads.Bead{Metadata: map[string]string{"gc.routed_to": "myrig/worker"}}
	if got := WorkflowExecutionRoute(b); got != "myrig/worker" {
		t.Errorf("got %q, want myrig/worker", got)
	}
}

func TestWorkflowExecutionRouteFromMeta_PrefersExecutionKey(t *testing.T) {
	meta := map[string]string{
		GraphExecutionRouteMetaKey: "executor",
		"gc.routed_to":             "control",
	}
	if got := WorkflowExecutionRouteFromMeta(meta); got != "executor" {
		t.Errorf("got %q, want executor (execution key takes precedence)", got)
	}
}
