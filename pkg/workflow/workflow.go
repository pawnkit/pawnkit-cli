// Package workflow runs deterministic command task graphs.
package workflow

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

type Task struct {
	Name      string
	DependsOn []string
	Run       func(context.Context) Result
}

type Result struct {
	Name     string    `json:"name"`
	Status   string    `json:"status"`
	Tool     string    `json:"tool,omitempty"`
	Message  string    `json:"message,omitempty"`
	Findings []Finding `json:"findings,omitempty"`
}

type Finding struct {
	RuleID   string `json:"ruleId"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Path     string `json:"path"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
}

type Options struct {
	Only        map[string]bool
	Skip        map[string]bool
	FailFast    bool
	Parallelism int
}

func Run(ctx context.Context, tasks []Task, opts Options) ([]Result, error) {
	known := make(map[string]Task, len(tasks))
	for _, task := range tasks {
		if task.Name == "" || task.Run == nil {
			return nil, errors.New("workflow: invalid task")
		}
		if _, exists := known[task.Name]; exists {
			return nil, fmt.Errorf("workflow: duplicate task %q", task.Name)
		}
		known[task.Name] = task
	}
	for _, task := range tasks {
		for _, dependency := range task.DependsOn {
			if _, exists := known[dependency]; !exists {
				return nil, fmt.Errorf("workflow: task %q has unknown dependency %q", task.Name, dependency)
			}
		}
	}
	selected := make(map[string]bool)
	var selectTask func(string) error
	selectTask = func(name string) error {
		task, ok := known[name]
		if !ok {
			return fmt.Errorf("workflow: unknown task %q", name)
		}
		if selected[name] {
			return nil
		}
		selected[name] = true
		for _, dependency := range task.DependsOn {
			if err := selectTask(dependency); err != nil {
				return err
			}
		}
		return nil
	}
	if len(opts.Only) == 0 {
		for name := range known {
			selected[name] = true
		}
	} else {
		for name := range opts.Only {
			if err := selectTask(name); err != nil {
				return nil, err
			}
		}
	}
	parallelism := opts.Parallelism
	if parallelism <= 0 || opts.FailFast {
		parallelism = 1
	}
	resultByIndex := make(map[int]Result, len(tasks))
	statuses := make(map[string]string, len(tasks))
	pending := make(map[int]bool, len(tasks))
	for index, task := range tasks {
		if !selected[task.Name] || opts.Skip[task.Name] {
			if opts.Skip[task.Name] {
				statuses[task.Name] = "skipped"
			}
			continue
		}
		pending[index] = true
	}
	stop := false
	for len(pending) != 0 && !stop {
		if err := ctx.Err(); err != nil {
			return orderedResults(tasks, resultByIndex), err
		}
		var ready []int
		progress := false
		for index, task := range tasks {
			if !pending[index] {
				continue
			}
			blocked, waiting := "", false
			for _, dependency := range task.DependsOn {
				status, complete := statuses[dependency]
				if !complete {
					waiting = true
					break
				}
				if status != "passed" {
					blocked = dependency
					break
				}
			}
			if blocked != "" {
				result := Result{Name: task.Name, Status: "failed", Message: fmt.Sprintf("dependency %s did not pass", blocked)}
				resultByIndex[index] = result
				statuses[task.Name] = result.Status
				delete(pending, index)
				progress = true
				if opts.FailFast {
					stop = true
				}
				continue
			}
			if !waiting {
				ready = append(ready, index)
			}
		}
		if stop {
			break
		}
		if len(ready) == 0 {
			if progress {
				continue
			}
			return orderedResults(tasks, resultByIndex), errors.New("workflow: dependency cycle")
		}
		if len(ready) > parallelism {
			ready = ready[:parallelism]
		}
		batch := make([]Result, len(ready))
		var wait sync.WaitGroup
		for batchIndex, taskIndex := range ready {
			wait.Go(func() {
				result := tasks[taskIndex].Run(ctx)
				result.Name = tasks[taskIndex].Name
				batch[batchIndex] = result
			})
		}
		wait.Wait()
		for batchIndex, taskIndex := range ready {
			result := batch[batchIndex]
			resultByIndex[taskIndex] = result
			statuses[tasks[taskIndex].Name] = result.Status
			delete(pending, taskIndex)
			if opts.FailFast && result.Status == "failed" {
				stop = true
				break
			}
		}
	}
	return orderedResults(tasks, resultByIndex), nil
}

func orderedResults(tasks []Task, results map[int]Result) []Result {
	ordered := make([]Result, 0, len(results))
	for index := range tasks {
		if result, ok := results[index]; ok {
			ordered = append(ordered, result)
		}
	}
	return ordered
}
