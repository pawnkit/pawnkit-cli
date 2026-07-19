package workflow_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/pawnkit/pawnkit-cli/pkg/workflow"
)

func TestRunOrderFilteringAndFailFast(t *testing.T) {
	var order []string
	task := func(name, status string) workflow.Task {
		return workflow.Task{Name: name, Run: func(context.Context) workflow.Result {
			order = append(order, name)
			return workflow.Result{Status: status}
		}}
	}
	results, err := workflow.Run(context.Background(), []workflow.Task{
		task("project", "passed"), task("format", "failed"), task("lint", "passed"),
	}, workflow.Options{Skip: map[string]bool{"project": true}, FailFast: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || len(order) != 1 || order[0] != "format" {
		t.Fatalf("results=%+v order=%v", results, order)
	}
}

func TestRunBoundedParallelismAndResultOrder(t *testing.T) {
	var active, maximum atomic.Int32
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	parallel := func(name string) workflow.Task {
		return workflow.Task{Name: name, DependsOn: []string{"project"}, Run: func(context.Context) workflow.Result {
			current := active.Add(1)
			for current > maximum.Load() && !maximum.CompareAndSwap(maximum.Load(), current) {
			}
			started <- struct{}{}
			<-release
			active.Add(-1)
			return workflow.Result{Status: "passed"}
		}}
	}
	tasks := []workflow.Task{
		{Name: "project", Run: func(context.Context) workflow.Result { return workflow.Result{Status: "passed"} }},
		parallel("format"), parallel("lint"),
	}
	done := make(chan []workflow.Result)
	go func() {
		results, _ := workflow.Run(context.Background(), tasks, workflow.Options{Parallelism: 2})
		done <- results
	}()
	<-started
	<-started
	close(release)
	results := <-done
	if maximum.Load() != 2 || len(results) != 3 || results[1].Name != "format" || results[2].Name != "lint" {
		t.Fatalf("maximum=%d results=%+v", maximum.Load(), results)
	}
}

func TestOnlyIncludesDependencies(t *testing.T) {
	var order []string
	tasks := []workflow.Task{
		{Name: "project", Run: func(context.Context) workflow.Result {
			order = append(order, "project")
			return workflow.Result{Status: "passed"}
		}},
		{Name: "lint", DependsOn: []string{"project"}, Run: func(context.Context) workflow.Result {
			order = append(order, "lint")
			return workflow.Result{Status: "passed"}
		}},
	}
	_, err := workflow.Run(context.Background(), tasks, workflow.Options{Only: map[string]bool{"lint": true}})
	if err != nil || len(order) != 2 || order[0] != "project" || order[1] != "lint" {
		t.Fatalf("order=%v error=%v", order, err)
	}
}

func TestRunCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := workflow.Run(ctx, []workflow.Task{{Name: "task", Run: func(context.Context) workflow.Result {
		return workflow.Result{Status: "passed"}
	}}}, workflow.Options{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
}

func TestRunRejectsUnknownDependency(t *testing.T) {
	_, err := workflow.Run(context.Background(), []workflow.Task{{
		Name: "test", DependsOn: []string{"build"},
		Run: func(context.Context) workflow.Result { return workflow.Result{Status: "passed"} },
	}}, workflow.Options{})
	if err == nil {
		t.Fatal("unknown dependency accepted")
	}
}
