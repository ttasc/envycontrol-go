// builder_test.go
package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestBuildTransactionPlan(t *testing.T) {
	defaultOpts := SwitchOptions{NvidiaModule: "nvidia"}

	tests := []struct {
		name       string
		target     string
		pciBus     string
		igpuVendor string
		opts       SwitchOptions
		wantErr    bool
		checkPlan  func(t *testing.T, plan TransactionPlan)
	}{
		{
			name:   "Integrated Mode",
			target: "integrated",
			checkPlan: func(t *testing.T, plan TransactionPlan) {
				if len(plan.ToCreate) != 2 {
					t.Fatalf("Expected 2 files created for integrated, got %d", len(plan.ToCreate))
				}
				if plan.ToCreate[0].Path != BlacklistPath {
					t.Errorf("Expected Blacklist path")
				}
				if !strings.Contains(plan.ToCreate[0].Content, "blacklist nvidia") {
					t.Errorf("Blacklist content missing nvidia module")
				}
			},
		},
		{
			name:   "Hybrid Mode (No RTD3)",
			target: "hybrid",
			opts:   defaultOpts,
			checkPlan: func(t *testing.T, plan TransactionPlan) {
				if len(plan.ToCreate) != 1 {
					t.Fatalf("Expected 1 file for basic hybrid, got %d", len(plan.ToCreate))
				}
				if !strings.Contains(plan.ToCreate[0].Content, "modeset=1") {
					t.Errorf("Expected modeset=1 in hybrid config")
				}
				if strings.Contains(plan.ToCreate[0].Content, "NVreg_DynamicPowerManagement") {
					t.Errorf("Did not expect RTD3 power management string")
				}
			},
		},
		{
			name:   "Hybrid Mode (With RTD3)",
			target: "hybrid",
			opts: func() SwitchOptions {
				rtd3 := 2
				return SwitchOptions{NvidiaModule: "nvidia", Rtd3Value: &rtd3}
			}(),
			checkPlan: func(t *testing.T, plan TransactionPlan) {
				if len(plan.ToCreate) != 2 {
					t.Fatalf("Expected 2 files for hybrid+rtd3, got %d", len(plan.ToCreate))
				}
				modesetFound := false
				for _, f := range plan.ToCreate {
					if strings.Contains(f.Content, "NVreg_DynamicPowerManagement=0x02") {
						modesetFound = true
					}
				}
				if !modesetFound {
					t.Errorf("RTD3 modeset value not found in plan")
				}
			},
		},
		{
			name:       "Nvidia Mode (Intel iGPU)",
			target:     "nvidia",
			pciBus:     "PCI:1:0:0",
			igpuVendor: "intel",
			opts:       defaultOpts,
			checkPlan: func(t *testing.T, plan TransactionPlan) {
				xorgFound := false
				for _, f := range plan.ToCreate {
					if f.Path == XorgPath {
						xorgFound = true
						if !strings.Contains(f.Content, "Inactive \"intel\"") {
							t.Errorf("Xorg config missing intel iGPU configuration")
						}
						if !strings.Contains(f.Content, "BusID \"PCI:1:0:0\"") {
							t.Errorf("Xorg config missing correct PCI Bus ID")
						}
					}
				}
				if !xorgFound {
					t.Errorf("Xorg config not created for Nvidia mode")
				}
			},
		},
		{
			name:    "Nvidia Mode (Missing PCI Bus)",
			target:  "nvidia",
			wantErr: true,
		},
		{
			name:    "Unknown Mode",
			target:  "super-gaming-mode",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan, err := BuildTransactionPlan(tt.target, tt.pciBus, tt.igpuVendor, tt.opts)
			if (err != nil) != tt.wantErr {
				t.Fatalf("BuildTransactionPlan() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && tt.checkPlan != nil {
				tt.checkPlan(t, plan)
			}

			// Always verify the removal footprint is constant
			if !tt.wantErr && !reflect.DeepEqual(plan.ToRemove, []string{
				BlacklistPath, UdevIntegratedPath, UdevPmPath, XorgPath, ModesetPath,
			}) {
				t.Errorf("ToRemove paths modified unexpectedly")
			}
		})
	}
}
