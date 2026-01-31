package sorter

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestPlanner_AddAction(t *testing.T) {
	tests := []struct {
		name   string
		plan   Plan
		action Action
		want   Plan
	}{
		{
			name: "add action to empty plan",
			plan: Plan{},
			action: Action{
				SourcePath: filepath.Join("user", "source", "file.txt"),
				DestDir:    "documents",
				DestPath:   filepath.Join("home", "dest", "documents"),
				FileSize:   128,
			},
			want: Plan{
				Actions: []Action{
					{
						SourcePath: filepath.Join("user", "source", "file.txt"),
						DestDir:    "documents",
						DestPath:   filepath.Join("home", "dest", "documents"),
						FileSize:   128,
					},
				},
				BaseDir: filepath.Join("user", "source"),
				DirCounts: map[string]int{
					"documents": 1,
				},
				DirSizes: map[string]int64{
					"documents": 128,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.plan.AddAction(tt.action)
			if got := tt.plan; !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Plan.AddAction() = %v, want %v", got, tt.want)
			}
		})
	}
}
