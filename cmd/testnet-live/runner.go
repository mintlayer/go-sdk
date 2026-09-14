// Copyright (c) 2026 Mintlayer Institutional FZCO
// Contact: hello@mintlayer.org
//
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

// Command testnet-live validates the Mintlayer Go SDK surface against the live
// local testnet (node + wallet daemons running in docker).
//
// It self-provisions fixtures (throwaway wallet file, freezable token,
// sacrificial DEX order), runs ordered checks per area and prints a
// PASS/FAIL/SKIP summary with the total fee spend. SDK defects discovered
// against the live daemon are reported as findings, never papered over.
package main

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Status is the outcome of a single check.
type Status string

const (
	StatusPass Status = "PASS"
	StatusFail Status = "FAIL"
	StatusSkip Status = "SKIP"
)

// Check is one validated assertion against the SDK / live daemons.
type Check struct {
	Area   string
	Name   string
	Status Status
	Detail string
	Fee    string // atoms spent by this check, if any
}

// Runner collects checks in execution order.
type Runner struct {
	checks []Check
	// SpentAtoms accumulates the total on-chain fee spend (big.Int decimal string).
	SpentAtoms string
	// BudgetAtoms is the fee guard; 0 disables the guard.
	BudgetAtoms string
	// findings are notable SDK/daemon defects surfaced during the run.
	Findings []string
}

func NewRunner(budgetAtoms string) *Runner {
	return &Runner{SpentAtoms: "0", BudgetAtoms: budgetAtoms}
}

// Record appends a check result.
func (r *Runner) Record(area, name string, status Status, detail string) {
	r.checks = append(r.checks, Check{Area: area, Name: name, Status: status, Detail: detail})
}

// RecordFee appends a check result and accounts the fee spent by it.
func (r *Runner) RecordFee(area, name string, status Status, detail, feeAtoms string) {
	r.checks = append(r.checks, Check{Area: area, Name: name, Status: status, Detail: detail, Fee: feeAtoms})
	if feeAtoms != "" {
		r.SpentAtoms = addAtoms(r.SpentAtoms, feeAtoms)
	}
}

// Finding registers a notable SDK/daemon defect discovered by the run.
func (r *Runner) Finding(format string, args ...any) {
	r.Findings = append(r.Findings, fmt.Sprintf(format, args...))
}

// OverBudget reports whether spending moreAtoms would exceed the budget.
func (r *Runner) OverBudget(moreAtoms string) bool {
	if r.BudgetAtoms == "" || r.BudgetAtoms == "0" {
		return false
	}
	return cmpAtoms(addAtoms(r.SpentAtoms, moreAtoms), r.BudgetAtoms) > 0
}

// sanitize collapses a message to a single printable line.
func sanitize(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 200 {
		s = s[:197] + "..."
	}
	return s
}

// Summary prints the grouped PASS/FAIL/SKIP matrix, fee spend and findings,
// and returns the process exit code (0 when no FAIL, 2 on infra failure).
func (r *Runner) Summary() int {
	byArea := map[string][]Check{}
	var areas []string
	for _, c := range r.checks {
		if _, ok := byArea[c.Area]; !ok {
			areas = append(areas, c.Area)
		}
		byArea[c.Area] = append(byArea[c.Area], c)
	}
	sort.Strings(areas)

	pass, fail, skip := 0, 0, 0
	for _, c := range r.checks {
		switch c.Status {
		case StatusPass:
			pass++
		case StatusFail:
			fail++
		case StatusSkip:
			skip++
		}
	}

	w := 110
	fmt.Println(strings.Repeat("=", w))
	fmt.Println(" TESTNET-LIVE RESULT MATRIX")
	fmt.Println(strings.Repeat("=", w))
	for _, area := range areas {
		fmt.Printf("-- %s --\n", area)
		for _, c := range byArea[area] {
			detail := c.Detail
			if c.Fee != "" {
				detail = fmt.Sprintf("%s [fee %s atoms]", detail, c.Fee)
			}
			if len(detail) > 72 {
				detail = detail[:69] + "..."
			}
			fmt.Printf(" %-4s | %-9s | %-38s | %s\n", c.Status, c.Area, c.Name, detail)
		}
	}
	fmt.Println(strings.Repeat("=", w))
	failed := 0
	for _, c := range r.checks {
		if c.Status == StatusFail && c.Detail != "" {
			failed++
			fmt.Printf(" FAIL DETAIL [%s/%s]: %s\n", c.Area, c.Name, sanitize(c.Detail))
		}
	}
	_ = failed
	fmt.Println(strings.Repeat("=", w))
	fmt.Printf(" TOTAL: %d PASS, %d FAIL, %d SKIP  (started %s)\n",
		pass, fail, skip, time.Now().Format(time.RFC3339))
	fmt.Printf(" FEE SPEND: %s atoms (%s ML) of budget %s atoms\n",
		r.SpentAtoms, atomsToML(r.SpentAtoms), r.BudgetAtoms)
	if len(r.Findings) > 0 {
		fmt.Println(strings.Repeat("-", w))
		fmt.Println(" SDK / DAEMON FINDINGS:")
		for i, f := range r.Findings {
			fmt.Printf("  %d. %s\n", i+1, sanitize(f))
		}
	}
	fmt.Println(strings.Repeat("=", w))
	if fail > 0 {
		return 1
	}
	return 0
}
